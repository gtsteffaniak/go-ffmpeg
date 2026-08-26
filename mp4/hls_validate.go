package mp4

import (
	"fmt"
	"math"
)

const (
	// DefaultHLSTimeToleranceSec is the default allowed drift for HLS timeline checks.
	DefaultHLSTimeToleranceSec = 0.050
	// MinHLSSegmentMediaBytes rejects empty fMP4 fragments.
	MinHLSSegmentMediaBytes = 8192
)

// SegmentTimeline describes decode timeline placement for one fMP4 media fragment.
type SegmentTimeline struct {
	Index            int
	MediaStartSec    float64
	ActualDurSec     float64
	ExpectedStartSec float64
	ExpectedDurSec   float64
	Bytes            int
}

// TimelineIssue records one validation failure.
type TimelineIssue struct {
	Check    string  `json:"check"`
	Message  string  `json:"message"`
	DeltaSec float64 `json:"deltaSec,omitempty"`
}

// ValidateSegmentTimeline checks one encoded media fragment against expected HLS placement.
func ValidateSegmentTimeline(media []byte, seg SegmentTimeline, toleranceSec float64) []TimelineIssue {
	return ValidateSegmentTimelineWithTimescales(media, seg, toleranceSec, nil)
}

// ValidateSegmentTimelineWithTimescales is like ValidateSegmentTimeline but interprets
// tfdt ticks using init-segment mdhd timescales when provided.
func ValidateSegmentTimelineWithTimescales(media []byte, seg SegmentTimeline, toleranceSec float64, trackTimescales map[uint32]uint32) []TimelineIssue {
	if toleranceSec <= 0 {
		toleranceSec = DefaultHLSTimeToleranceSec
	}
	var issues []TimelineIssue
	if len(media) < MinHLSSegmentMediaBytes {
		issues = append(issues, TimelineIssue{
			Check:   "min_size",
			Message: fmt.Sprintf("segment %d too small (%d bytes)", seg.Index, len(media)),
		})
	}
	startSec, err := FragmentMediaStartSecWithTimescales(media, trackTimescales)
	if err != nil {
		issues = append(issues, TimelineIssue{
			Check:   "tfdt_start",
			Message: fmt.Sprintf("segment %d read tfdt: %v", seg.Index, err),
		})
	} else {
		seg.MediaStartSec = startSec
		delta := math.Abs(startSec - seg.ExpectedStartSec)
		if delta > toleranceSec {
			issues = append(issues, TimelineIssue{
				Check:    "tfdt_start",
				Message:  fmt.Sprintf("segment %d tfdt_start=%.3f expected=%.3f", seg.Index, startSec, seg.ExpectedStartSec),
				DeltaSec: delta,
			})
		}
	}
	actualDur := seg.ActualDurSec
	if actualDur <= 0 {
		actualDur = FragmentDurationSec(media)
	}
	if actualDur > 0 {
		seg.ActualDurSec = actualDur
		if seg.ExpectedDurSec > 0 {
			delta := math.Abs(actualDur - seg.ExpectedDurSec)
			if delta > toleranceSec {
				issues = append(issues, TimelineIssue{
					Check:    "duration_match",
					Message:  fmt.Sprintf("segment %d actualDur=%.3f expected=%.3f", seg.Index, actualDur, seg.ExpectedDurSec),
					DeltaSec: delta,
				})
			}
		}
	}
	if err := validateMonotonicTFDT(media); err != nil {
		issues = append(issues, TimelineIssue{
			Check:   "monotonic",
			Message: fmt.Sprintf("segment %d: %v", seg.Index, err),
		})
	}
	return issues
}

// ValidateContinuity checks that segment N ends where segment N+1 begins.
func ValidateContinuity(prev, next SegmentTimeline, toleranceSec float64) []TimelineIssue {
	if toleranceSec <= 0 {
		toleranceSec = DefaultHLSTimeToleranceSec
	}
	if prev.ActualDurSec <= 0 || next.MediaStartSec <= 0 {
		return nil
	}
	prevEnd := prev.MediaStartSec + prev.ActualDurSec
	delta := math.Abs(prevEnd - next.MediaStartSec)
	if delta > toleranceSec {
		gap := next.MediaStartSec - prevEnd
		kind := "gap"
		if gap < 0 {
			kind = "overlap"
		}
		return []TimelineIssue{{
			Check:    "continuity",
			Message:  fmt.Sprintf("segment %d→%d %s=%.3fs (prevEnd=%.3f nextStart=%.3f)", prev.Index, next.Index, kind, math.Abs(gap), prevEnd, next.MediaStartSec),
			DeltaSec: delta,
		}}
	}
	return nil
}

// FragmentMediaStartSec returns the first video track baseMediaDecodeTime in seconds
// using the default 90 kHz video timescale.
func FragmentMediaStartSec(media []byte) (float64, error) {
	return FragmentMediaStartSecWithTimescales(media, nil)
}

// FragmentMediaStartSecWithTimescales returns the first video track decode time in
// seconds using init-segment mdhd timescales when provided. ffmpeg HLS fMP4 often
// writes a video timescale other than 90000 (e.g. 12288).
func FragmentMediaStartSecWithTimescales(media []byte, trackTimescales map[uint32]uint32) (float64, error) {
	firstByTrack, err := firstTFDTByTrack(media)
	if err != nil {
		return 0, err
	}
	trackID := uint32(1)
	ticks, ok := firstByTrack[1]
	if !ok {
		for id, v := range firstByTrack {
			trackID = id
			ticks = v
			ok = true
			break
		}
	}
	if !ok {
		return 0, fmt.Errorf("no tfdt in fragment")
	}
	ts := timescaleForTrackID(trackID, trackTimescales)
	if ts == 0 {
		ts = defaultVideoTimescale
	}
	return float64(ticks) / float64(ts), nil
}

func validateMonotonicTFDT(media []byte) error {
	var last uint64
	var seen bool
	err := walkAtoms(media, 0, len(media), func(off, size int, typ string) error {
		if typ != "moof" {
			return nil
		}
		end := off + size
		return walkAtoms(media, off+8, end, func(trafOff, trafSize int, trafTyp string) error {
			if trafTyp != "traf" {
				return nil
			}
			trafEnd := trafOff + trafSize
			trackID, err := tfhdTrackID(media, trafOff, trafEnd)
			if err != nil || trackID != 1 {
				return err
			}
			return walkAtoms(media, trafOff+8, trafEnd, func(atomOff, atomSize int, atomTyp string) error {
				if atomTyp != "tfdt" {
					return nil
				}
				ticks, err := readTFDTTicks(media, atomOff, atomSize)
				if err != nil {
					return err
				}
				if seen && ticks < last {
					return fmt.Errorf("tfdt decreased %d -> %d", last, ticks)
				}
				last = ticks
				seen = true
				return nil
			})
		})
	})
	return err
}

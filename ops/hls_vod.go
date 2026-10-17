package ops

import (
	"fmt"
	"math"
	"strings"
)

// VOD HLS layout constants (Model A). Hosts serve these paths from a cache job directory.
const (
	HLSVODInitFilename       = "init.m4s"
	HLSVODSegmentSubdir      = "seg"
	HLSVODSegmentNamePattern = "seg/%05d.m4s"
	// Internal ffmpeg playlist; hosts must not expose this (they serve BuildVODPlaylist output).
	HLSVODInternalPlaylistName = ".ffmpeg-vod.m3u8"
)

// VODSegmentList is the segment timeline for a full-length VOD media playlist.
type VODSegmentList struct {
	Starts     []float64
	Durations  []float64
	SegmentSec float64 // target segment length (grid for transcode; snap target for copy)
}

// Count returns the number of segments in the list.
func (l VODSegmentList) Count() int {
	return len(l.Starts)
}

// BuildVODSegmentListFixedGrid plans equal segments on a 0-based media timeline (full transcode).
func BuildVODSegmentListFixedGrid(mediaDurationSec, segmentSec float64) VODSegmentList {
	if segmentSec <= 0 {
		segmentSec = DefaultHLSSegmentDurationSec
	}
	starts, durs := fixedHLSSegmentTimeline(mediaDurationSec, segmentSec)
	return VODSegmentList{Starts: starts, Durations: durs, SegmentSec: segmentSec}
}

// BuildVODSegmentListKeyframe plans segments snapped to probed keyframes (remux / video-copy).
// Keyframes and duration are normalized when startTimeSec is non-zero.
func BuildVODSegmentListKeyframe(mediaDurationSec, segmentSec, startTimeSec float64, keyframes []float64) VODSegmentList {
	if segmentSec <= 0 {
		segmentSec = DefaultHLSSegmentDurationSec
	}
	kf, dur := PrepareHLSMediaTimeline(keyframes, mediaDurationSec, startTimeSec)
	starts, durs := BuildHLSSegmentTimeline(dur, kf, segmentSec, 0)
	return VODSegmentList{Starts: starts, Durations: durs, SegmentSec: segmentSec}
}

// BuildVODSegmentList selects fixed-grid or keyframe mode from keyframes (nil/empty → grid).
func BuildVODSegmentList(mediaDurationSec, segmentSec, startTimeSec float64, keyframes []float64) VODSegmentList {
	kf, dur := PrepareHLSMediaTimeline(keyframes, mediaDurationSec, startTimeSec)
	if len(kf) == 0 {
		return BuildVODSegmentListFixedGrid(dur, segmentSec)
	}
	return BuildVODSegmentListKeyframe(mediaDurationSec, segmentSec, startTimeSec, keyframes)
}

// SegmentIndexForTime maps a playlist time in seconds to a segment index and that segment's start time.
// Times before the first segment snap to index 0; times past the end snap to the last segment.
func SegmentIndexForTime(timeSec float64, starts []float64) (index int, startSec float64) {
	if len(starts) == 0 {
		return 0, 0
	}
	if timeSec <= starts[0] {
		return 0, starts[0]
	}
	idx := 0
	for i := 1; i < len(starts); i++ {
		if starts[i] > timeSec+0.0005 {
			break
		}
		idx = i
	}
	return idx, starts[idx]
}

// VODPlaylistOptions configures URIs in a synthetic VOD media playlist.
type VODPlaylistOptions struct {
	InitURI string
	// SegmentURI returns the playlist URI for segment index i (e.g. "seg/00012.m4s").
	SegmentURI func(index int) string
}

// DefaultVODPlaylistOptions returns init and segment URIs using the standard layout.
func DefaultVODPlaylistOptions() VODPlaylistOptions {
	return VODPlaylistOptions{
		InitURI: HLSVODInitFilename,
		SegmentURI: func(i int) string {
			return fmt.Sprintf(HLSVODSegmentNamePattern, i)
		},
	}
}

// VODTargetDurationSec returns #EXT-X-TARGETDURATION from segment durations (ceiling of max EXTINF).
func VODTargetDurationSec(durations []float64) int {
	max := 0.0
	for _, d := range durations {
		if d > max {
			max = d
		}
	}
	if max <= 0 {
		max = DefaultHLSSegmentDurationSec
	}
	return int(math.Ceil(max - 1e-9))
}

// BuildVODPlaylist renders a complete #EXT-X-PLAYLIST-TYPE:VOD media playlist for Model A.
// The host serves this regardless of encode progress; segment URIs must match on-disk names.
func BuildVODPlaylist(list VODSegmentList, opts VODPlaylistOptions) string {
	if opts.InitURI == "" {
		opts.InitURI = HLSVODInitFilename
	}
	if opts.SegmentURI == nil {
		def := DefaultVODPlaylistOptions()
		opts.SegmentURI = def.SegmentURI
	}
	target := VODTargetDurationSec(list.Durations)
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:7\n")
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", target))
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%s\"\n", opts.InitURI))
	for i := 0; i < list.Count(); i++ {
		dur := list.Durations[i]
		if dur <= 0 {
			dur = list.SegmentSec
		}
		b.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", dur))
		b.WriteString(opts.SegmentURI(i))
		b.WriteByte('\n')
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

// HLSVODSegmentRelPath returns the relative segment path for index (under the job directory).
func HLSVODSegmentRelPath(index int) string {
	return fmt.Sprintf(HLSVODSegmentNamePattern, index)
}

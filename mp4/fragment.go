package mp4

import (
	"encoding/binary"
	"math"
)

// FragmentDurationSec returns the span of decode time covered by fMP4 media,
// using the longest track span in the fragment (video typically dominates).
func FragmentDurationSec(media []byte) float64 {
	return FragmentDurationSecWithTimescales(media, nil)
}

func trafDurationTicks(data []byte, trafOff, trafEnd int) (uint64, bool) {
	var defaultDur uint32
	var hasDefault bool
	_ = walkAtoms(data, trafOff+8, trafEnd, func(atomOff, atomSize int, atomTyp string) error {
		if atomTyp != "tfhd" {
			return nil
		}
		d, ok := tfhdDefaultSampleDuration(data, atomOff, atomSize)
		if ok {
			defaultDur = d
			hasDefault = true
		}
		return nil
	})

	var total uint64
	found := false
	_ = walkAtoms(data, trafOff+8, trafEnd, func(atomOff, atomSize int, atomTyp string) error {
		if atomTyp != "trun" {
			return nil
		}
		ticks, ok := trunDurationTicks(data, atomOff, atomSize, defaultDur, hasDefault)
		if ok {
			total += ticks
			found = true
		}
		return nil
	})
	return total, found
}

// inferTrackDurationSec maps trun tick sums to seconds when mdhd timescale is not available
// (e.g. media-only fMP4 fragments). Picks the candidate timescale that yields a plausible
// HLS segment duration (0.5–12s), preferring the longest valid span (usually video).
func inferTrackDurationSec(ticks uint64) float64 {
	if ticks == 0 {
		return 0
	}
	order := []uint32{90000, 16000, 48000, 44100, 30000, 1000}
	for _, ts := range order {
		sec := float64(ticks) / float64(ts)
		if sec >= 0.5 && sec <= 12.0 {
			return sec
		}
	}
	return float64(ticks) / float64(defaultVideoTimescale)
}

func tfhdDefaultSampleDuration(data []byte, atomOff, atomSize int) (uint32, bool) {
	if atomOff+16 > len(data) || atomSize < 16 {
		return 0, false
	}
	flags := uint32(data[atomOff+9])<<16 | uint32(data[atomOff+10])<<8 | uint32(data[atomOff+11])
	if flags&0x8 == 0 {
		return 0, false
	}
	cursor := atomOff + 16
	if flags&0x1 != 0 {
		cursor += 8
	}
	if flags&0x2 != 0 {
		cursor += 4
	}
	if cursor+4 > atomOff+atomSize || cursor+4 > len(data) {
		return 0, false
	}
	return binary.BigEndian.Uint32(data[cursor : cursor+4]), true
}

func trunDurationTicks(data []byte, atomOff, atomSize int, defaultDur uint32, hasDefault bool) (uint64, bool) {
	if atomOff+16 > len(data) || atomSize < 16 {
		return 0, false
	}
	flags := uint32(data[atomOff+9])<<16 | uint32(data[atomOff+10])<<8 | uint32(data[atomOff+11])
	sampleCount := int(binary.BigEndian.Uint32(data[atomOff+12 : atomOff+16]))
	if sampleCount <= 0 {
		return 0, false
	}
	cursor := atomOff + 16
	trafEnd := atomOff + atomSize
	if flags&0x1 != 0 {
		cursor += 4
	}
	if flags&0x4 != 0 {
		cursor += 4
	}
	hasPerSample := flags&0x100 != 0
	if !hasPerSample {
		if !hasDefault || defaultDur == 0 {
			return 0, false
		}
		return uint64(defaultDur) * uint64(sampleCount), true
	}
	var total uint64
	for i := 0; i < sampleCount; i++ {
		if cursor+4 > trafEnd || cursor+4 > len(data) {
			return 0, false
		}
		total += uint64(binary.BigEndian.Uint32(data[cursor : cursor+4]))
		cursor += 4
		if flags&0x200 != 0 {
			cursor += 4
		}
		if flags&0x400 != 0 {
			cursor += 4
		}
		if flags&0x800 != 0 {
			cursor += 4
		}
	}
	return total, total > 0
}

// RoundDurationSec rounds to millisecond precision for HLS EXTINF tags.
func RoundDurationSec(sec float64) float64 {
	if sec <= 0 {
		return 0
	}
	return math.Round(sec*1000) / 1000
}

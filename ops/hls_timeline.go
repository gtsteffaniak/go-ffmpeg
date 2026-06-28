package ops

import "math"

const (
	hlsKeyframeMinGapSec     = 0.5
	hlsMinSegmentDurationSec = 0.5 // ffmpeg cannot encode shorter on-demand segments reliably
	// Reject only pathological indexes (e.g. every frame marked as a keyframe at 15+ Hz).
	hlsKeyframeMaxPerSec = 15.0
)

// SanitizeHLSKeyframes filters spurious keyframe probes from corrupt indexes.
// Returns nil when keyframes are unusable so callers fall back to a fixed grid.
func SanitizeHLSKeyframes(keyframes []float64, durationSec float64) []float64 {
	if len(keyframes) == 0 || durationSec <= 0 {
		return nil
	}
	if float64(len(keyframes)) > durationSec*hlsKeyframeMaxPerSec {
		return nil
	}

	out := make([]float64, 0, len(keyframes))
	for _, t := range keyframes {
		if t < 0 || t >= durationSec {
			continue
		}
		if len(out) == 0 {
			out = append(out, t)
			continue
		}
		if t-out[len(out)-1] < hlsKeyframeMinGapSec {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	if out[0] > 0.001 {
		out = append([]float64{0}, out...)
	}
	return out
}

// BuildHLSSegmentTimeline returns segment start times and durations in seconds.
// segmentDurationSec is the target grid length (typically 4s for on-demand fMP4).
func BuildHLSSegmentTimeline(durationSec float64, keyframes []float64, segmentDurationSec float64) (starts, durations []float64) {
	if segmentDurationSec <= 0 {
		segmentDurationSec = DefaultHLSSegmentDurationSec
	}
	if durationSec <= 0 {
		durationSec = segmentDurationSec
	}
	if len(keyframes) == 0 {
		return fixedHLSSegmentTimeline(durationSec, segmentDurationSec)
	}

	kf := append([]float64(nil), keyframes...)
	if kf[0] > 0.001 {
		kf = append([]float64{0}, kf...)
	}

	for i := 0; i < len(kf); {
		start := kf[i]
		if start >= durationSec {
			break
		}
		targetEnd := start + segmentDurationSec
		j := i + 1
		for j < len(kf) && kf[j] < targetEnd {
			j++
		}
		end := durationSec
		if j < len(kf) {
			end = kf[j]
		}
		dur := end - start
		if dur <= 0.01 {
			i++
			continue
		}
		starts = append(starts, start)
		durations = append(durations, dur)
		if end >= durationSec-0.001 {
			break
		}
		i = j
	}
	if len(starts) == 0 {
		return fixedHLSSegmentTimeline(durationSec, segmentDurationSec)
	}
	return mergeTinyTailSegment(starts, durations)
}

func fixedHLSSegmentTimeline(durationSec, segmentDurationSec float64) (starts, durations []float64) {
	count := int(math.Ceil(durationSec / segmentDurationSec))
	if count < 1 {
		count = 1
	}
	for i := 0; i < count; i++ {
		start := float64(i) * segmentDurationSec
		dur := segmentDurationSec
		if rem := durationSec - start; rem > 0 && rem < dur {
			dur = rem
		}
		if dur <= 0 {
			break
		}
		starts = append(starts, start)
		durations = append(durations, dur)
	}
	return mergeTinyTailSegment(starts, durations)
}

func mergeTinyTailSegment(starts, durations []float64) ([]float64, []float64) {
	if len(durations) < 2 || durations[len(durations)-1] >= hlsMinSegmentDurationSec {
		return starts, durations
	}
	durations[len(durations)-2] += durations[len(durations)-1]
	return starts[:len(starts)-1], durations[:len(durations)-1]
}

// KeyframeSeekBefore returns the largest keyframe time <= sec, or 0 when none.
func KeyframeSeekBefore(keyframes []float64, sec float64) float64 {
	if len(keyframes) == 0 || sec <= 0 {
		return 0
	}
	seek := 0.0
	for _, kf := range keyframes {
		if kf > sec+0.001 {
			break
		}
		seek = kf
	}
	return seek
}

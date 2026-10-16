package ops

import (
	"math"
	"testing"
)

func TestBuildHLSSegmentTimelineMergesTinyTail(t *testing.T) {
	t.Parallel()
	starts, durs := BuildHLSSegmentTimeline(32.1, nil, DefaultHLSSegmentDurationSec, 0)
	if len(starts) != 8 {
		t.Fatalf("starts len = %d, want 8 (tiny tail merged)", len(starts))
	}
	if starts[7] != 28 || durs[7] < 4.0 || durs[7] > 4.2 {
		t.Fatalf("last segment = %.3f/%.3f, want start 28 and dur ~4.1", starts[7], durs[7])
	}
}

func TestBuildHLSSegmentTimelineFixedGrid(t *testing.T) {
	t.Parallel()
	starts, durs := BuildHLSSegmentTimeline(10, nil, DefaultHLSSegmentDurationSec, 0)
	if len(starts) != 3 {
		t.Fatalf("starts len = %d, want 3", len(starts))
	}
	if starts[0] != 0 || durs[0] != 4 || starts[2] != 8 || durs[2] != 2 {
		t.Fatalf("unexpected timeline: starts=%v durs=%v", starts, durs)
	}
}

func TestSanitizeHLSKeyframesRejectsDenseProbes(t *testing.T) {
	t.Parallel()
	dense := make([]float64, 3000)
	for i := range dense {
		dense[i] = float64(i) / 30.0
	}
	if got := SanitizeHLSKeyframes(dense, 100); got != nil {
		t.Fatalf("expected nil for dense corrupt keyframes, got len=%d", len(got))
	}
}

func TestHLSSegmentGOP(t *testing.T) {
	t.Parallel()
	gop := HLSSegmentGOP(30, DefaultOnDemandHLSDefaults())
	if gop != 120 {
		t.Fatalf("gop = %d, want 120", gop)
	}
	gop = HLSSegmentGOP(0, DefaultOnDemandHLSDefaults())
	if gop != 120 {
		t.Fatalf("gop = %d, want default 120", gop)
	}
}

func TestNormalizeHLSKeyframeTimesAndStartTimeTimeline(t *testing.T) {
	t.Parallel()
	const start = 1.459
	raw := []float64{1.480, 2.480, 3.480}
	norm := NormalizeHLSKeyframeTimes(raw, start)
	if len(norm) != 3 || norm[0] < 0.019 || norm[0] > 0.023 {
		t.Fatalf("normalized keyframes = %v", norm)
	}
	starts, _ := BuildHLSSegmentTimeline(30, raw, DefaultHLSSegmentDurationSec, start)
	if len(starts) == 0 || starts[0] > 0.05 {
		t.Fatalf("first segment start = %.3f, want near media 0", starts[0])
	}
}

func TestHLSSegmentCount(t *testing.T) {
	t.Parallel()
	n := int(math.Ceil(120 / DefaultHLSSegmentDurationSec))
	if n != 30 {
		t.Fatalf("segment count = %d, want 30", n)
	}
}

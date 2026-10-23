package ops

import "testing"

func TestVODCopySegmentCutTimesRel(t *testing.T) {
	t.Parallel()
	kf := []float64{0, 6, 9, 12, 18, 21, 24}
	list := BuildVODSegmentListKeyframe(30, 4, 0, kf)
	times := VODCopySegmentCutTimesRel(list, 6, 1)
	want := []float64{3, 6, 12, 15, 18}
	if len(times) != len(want) {
		t.Fatalf("times=%v want %v", times, want)
	}
	for i := range want {
		if times[i] != want[i] {
			t.Fatalf("times[%d]=%.3f want %.3f", i, times[i], want[i])
		}
	}
}

func TestFloatsToArgStringsFloorsCutTimes(t *testing.T) {
	t.Parallel()
	// A cut at 10.0005 must not round up to 10.001 — that would land after the
	// 10.0005 keyframe and push the boundary to the next one.
	got := floatsToArgStrings([]float64{10.0005, 12})
	want := []string{"9.999", "11.999"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

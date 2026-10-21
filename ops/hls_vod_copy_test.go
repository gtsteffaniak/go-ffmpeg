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

package ops

import (
	"strings"
	"testing"
)

func TestBuildVODPlaylistFixedGrid(t *testing.T) {
	t.Parallel()
	list := BuildVODSegmentListFixedGrid(10.0, 4.0)
	playlist := BuildVODPlaylist(list, DefaultVODPlaylistOptions())
	for _, want := range []string{
		"#EXT-X-PLAYLIST-TYPE:VOD",
		"#EXT-X-TARGETDURATION:4",
		"#EXT-X-MAP:URI=\"init.m4s\"",
		"#EXTINF:4.000,",
		"seg/00000.m4s",
		"#EXTINF:2.000,",
		"seg/00002.m4s",
		"#EXT-X-ENDLIST",
	} {
		if !strings.Contains(playlist, want) {
			t.Fatalf("playlist missing %q:\n%s", want, playlist)
		}
	}
	if list.Count() != 3 {
		t.Fatalf("segment count = %d, want 3", list.Count())
	}
}

func TestSegmentIndexForTime(t *testing.T) {
	t.Parallel()
	list := BuildVODSegmentListFixedGrid(32.0, 4.0)
	cases := []struct {
		t     float64
		idx   int
		start float64
	}{
		{-1, 0, 0},
		{0, 0, 0},
		{3.99, 0, 0},
		{4, 1, 4},
		{15.5, 3, 12},
		{100, 7, 28},
	}
	for _, c := range cases {
		idx, start := SegmentIndexForTime(c.t, list.Starts)
		if idx != c.idx || start != c.start {
			t.Fatalf("time %.2f: got idx=%d start=%.1f want idx=%d start=%.1f", c.t, idx, start, c.idx, c.start)
		}
	}
}

func TestSegmentIndexForTimeKeyframeTail(t *testing.T) {
	t.Parallel()
	kf := []float64{0, 4, 8, 12, 16, 20, 24, 28}
	list := BuildVODSegmentListKeyframe(30.5, 4, 0, kf)
	idx, start := SegmentIndexForTime(29.9, list.Starts)
	if idx != list.Count()-1 {
		t.Fatalf("idx=%d want last %d", idx, list.Count()-1)
	}
	if start != list.Starts[idx] {
		t.Fatalf("start=%.3f want %.3f", start, list.Starts[idx])
	}
	lastDur := list.Durations[list.Count()-1]
	if lastDur < 0.5 {
		t.Fatalf("last duration %.3f too short", lastDur)
	}
}

func TestBuildVODSegmentListKeyframeMatchesFFmpegCumulative(t *testing.T) {
	t.Parallel()
	var kf []float64
	for i := 0; i <= 10; i++ {
		kf = append(kf, float64(i)*3)
	}
	list := BuildVODSegmentListKeyframe(30, 4, 0, kf)
	wantStarts := []float64{0, 6, 9, 12, 18, 21, 24}
	wantDurs := []float64{6, 3, 3, 6, 3, 3, 6}
	if list.Count() != len(wantStarts) {
		t.Fatalf("count=%d want %d starts=%v durs=%v", list.Count(), len(wantStarts), list.Starts, list.Durations)
	}
	for i := range wantStarts {
		if list.Starts[i] != wantStarts[i] || list.Durations[i] != wantDurs[i] {
			t.Fatalf("segment %d: got %.3f/%.3f want %.3f/%.3f", i, list.Starts[i], list.Durations[i], wantStarts[i], wantDurs[i])
		}
	}
}

func TestBuildVODSegmentListMergesTinyTail(t *testing.T) {
	t.Parallel()
	list := BuildVODSegmentListFixedGrid(8.3, 4.0)
	if list.Count() != 2 {
		t.Fatalf("count=%d want 2 (tiny tail merged)", list.Count())
	}
	if list.Durations[1] < 4 {
		t.Fatalf("last dur=%.3f", list.Durations[1])
	}
}

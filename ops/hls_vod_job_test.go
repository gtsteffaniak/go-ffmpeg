package ops

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVODAbsLocalInputURLPreservesRemoteSchemes(t *testing.T) {
	t.Parallel()
	for _, url := range []string{"", "http://host/video.mkv", "rtsp://cam/stream"} {
		if vodAbsLocalInputURL(url) != url {
			t.Fatalf("remote URL changed: %q", url)
		}
	}
}

func TestVODAbsLocalInputURLAbsolutesLocalPaths(t *testing.T) {
	t.Parallel()
	got := vodAbsLocalInputURL("clip.mkv")
	if strings.Contains(got, "://") {
		t.Fatalf("local path became URL: %q", got)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("expected absolute path, got %q", got)
	}
}

func writeNonReadyVODSegment(t *testing.T, dir string, index int) string {
	t.Helper()
	segDir := filepath.Join(dir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(segDir, fmt.Sprintf("%05d.m4s", index))
	if err := os.WriteFile(path, []byte("not a fragment"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClearStaleVODSegmentsStopsAtFirstReady(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	segDir := filepath.Join(dir, "seg")
	// Earlier coverage below the rewrite point is untouched.
	writeReadyVODSegment(t, dir, 0)
	writeReadyVODSegment(t, dir, 1)
	// Stale partial and a non-publishable segment inside the gap get removed.
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tmpPath := filepath.Join(segDir, "00002.m4s.tmp")
	if err := os.WriteFile(tmpPath, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeNonReadyVODSegment(t, dir, 3)
	writeNonReadyVODSegment(t, dir, 4)
	// Already-encoded tail survives: first ready segment bounds the rewrite.
	writeReadyVODSegment(t, dir, 5)
	writeReadyVODSegment(t, dir, 6)

	bound, err := clearStaleVODSegments(dir, HLSVODJobOptions{StartIndex: 2, SegmentSec: 4})
	if err != nil {
		t.Fatalf("clearStaleVODSegments: %v", err)
	}
	if bound != 5 {
		t.Fatalf("bound=%d want 5", bound)
	}
	for _, p := range []string{tmpPath, filepath.Join(segDir, "00003.m4s"), filepath.Join(segDir, "00004.m4s")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("stale file survived: %s", p)
		}
	}
	for _, idx := range []int{0, 1, 5, 6} {
		p := filepath.Join(segDir, fmt.Sprintf("%05d.m4s", idx))
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("segment %d removed: %v", idx, err)
		}
	}
}

func TestClearStaleVODSegmentsNoReadyTail(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeReadyVODSegment(t, dir, 0)
	writeNonReadyVODSegment(t, dir, 10)

	bound, err := clearStaleVODSegments(dir, HLSVODJobOptions{StartIndex: 5, SegmentSec: 4})
	if err != nil {
		t.Fatalf("clearStaleVODSegments: %v", err)
	}
	if bound != 0 {
		t.Fatalf("bound=%d want 0 when nothing ready follows", bound)
	}
}

func TestPrepareVODOutputLayoutBoundsJobAtReadySegment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeReadyVODSegment(t, dir, 7)
	opts := HLSVODJobOptions{StartIndex: 2, SegmentSec: 4, ClearSegmentsFromIndex: true}
	if err := prepareVODOutputLayout(dir, &opts); err != nil {
		t.Fatalf("prepareVODOutputLayout: %v", err)
	}
	if opts.StopIndex != 7 {
		t.Fatalf("StopIndex=%d want 7", opts.StopIndex)
	}
}

func TestVODJobStopDurationSec(t *testing.T) {
	t.Parallel()
	// Transcode grid: bound at (StopIndex-StartIndex)*SegmentSec minus guard.
	d := vodJobStopDurationSec(HLSVODJobOptions{StartIndex: 5, StopIndex: 8, StartSec: 20, SegmentSec: 4})
	if math.Abs(d-11.8) > 0.001 {
		t.Fatalf("transcode stop=%.3f want 11.800", d)
	}
	// Explicit copy: bound is the segment list start, based at the keyframe seek.
	kf := []float64{0, 6, 9, 12, 18, 21, 24}
	list := BuildVODSegmentListKeyframe(30, 4, 0, kf)
	d = vodJobStopDurationSec(HLSVODJobOptions{
		StartIndex: 1, StopIndex: 4, StartSec: 6, SegmentSec: 4,
		VideoCopy: true, KeyframeSeekTimes: kf, SegmentList: &list,
	})
	if math.Abs(d-11.8) > 0.001 {
		t.Fatalf("copy stop=%.3f want 11.800", d)
	}
	if vodJobStopDurationSec(HLSVODJobOptions{StartIndex: 5, SegmentSec: 4}) != 0 {
		t.Fatal("no StopIndex should mean unbounded")
	}
}

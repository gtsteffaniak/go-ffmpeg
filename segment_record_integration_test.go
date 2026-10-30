//go:build integration

package ffmpeg_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

// TestIntegrationSegmentRecordAutoCopiesCompatibleSource records the sample in
// Auto mode. An H.264 source must be stream-copied, not re-encoded, and the plan
// reported through OnPlan must match what happened on disk.
func TestIntegrationSegmentRecordAutoCopiesCompatibleSource(t *testing.T) {
	sample := integrationSampleMP4(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	svc := newIntegrationService(t, ctx)

	info, err := svc.ProbeFile(ctx, sample)
	if err != nil {
		t.Fatalf("probe sample: %v", err)
	}
	if strings.ToLower(info.VideoCodec) != "h264" {
		t.Skipf("sample video codec is %q; this test asserts the H.264 copy path", info.VideoCodec)
	}

	outDir := t.TempDir()
	var plan ffmpeg.SegmentPlan
	err = svc.SegmentRecord(ctx, ffmpeg.SegmentRecordOptions{
		Input:           ffmpeg.InputSource{URL: sample, StreamType: "file"},
		OutputDir:       outDir,
		SegmentPattern:  "seg_%03d.mp4",
		SegmentDuration: 4 * time.Second,
		Mode:            ffmpeg.RecordAuto,
		Fragmented:      true,
		OnPlan:          func(p ffmpeg.SegmentPlan) { plan = p },
	})
	if err != nil {
		t.Fatalf("SegmentRecord: %v", err)
	}
	if plan.Video != "copy" {
		t.Fatalf("auto plan video = %q, want copy (reason: %s)", plan.Video, plan.VideoReason)
	}

	segments := listSegments(t, outDir)
	if len(segments) < 2 {
		t.Fatalf("expected at least 2 segments for a 10s sample with 4s segments, got %d", len(segments))
	}
	var total float64
	for _, seg := range segments {
		d, err := svc.GetMediaDuration(ctx, seg)
		if err != nil {
			t.Fatalf("duration of %s: %v", seg, err)
		}
		total += d
	}
	if math.Abs(total-10) > 1.0 {
		t.Fatalf("segments total %.2fs, want about 10s", total)
	}
}

// TestIntegrationConcatMediaCopyAndRescale joins recorded segments. Matching
// inputs must copy; a rescale request must re-encode to the requested size.
func TestIntegrationConcatMediaCopyAndRescale(t *testing.T) {
	sample := integrationSampleMP4(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	svc := newIntegrationService(t, ctx)

	outDir := t.TempDir()
	if err := svc.SegmentRecord(ctx, ffmpeg.SegmentRecordOptions{
		Input:           ffmpeg.InputSource{URL: sample, StreamType: "file"},
		OutputDir:       outDir,
		SegmentPattern:  "part_%03d.mp4",
		SegmentDuration: 4 * time.Second,
		Mode:            ffmpeg.RecordCopy,
	}); err != nil {
		t.Fatalf("SegmentRecord: %v", err)
	}
	segments := listSegments(t, outDir)
	if len(segments) < 2 {
		t.Fatalf("need at least 2 segments, got %d", len(segments))
	}

	copied := filepath.Join(t.TempDir(), "joined.mp4")
	plan, err := svc.ConcatMedia(ctx, ffmpeg.ConcatMediaOptions{Inputs: segments, OutputPath: copied})
	if err != nil {
		t.Fatalf("ConcatMedia copy: %v", err)
	}
	if plan.Action != "copy" {
		t.Fatalf("matching segments plan = %q, want copy (%s)", plan.Action, plan.Reason)
	}
	if d, err := svc.GetMediaDuration(ctx, copied); err != nil || math.Abs(d-10) > 1.0 {
		t.Fatalf("joined duration = %.2f (err %v), want about 10s", d, err)
	}

	scaled := filepath.Join(t.TempDir(), "scaled.mp4")
	plan, err = svc.ConcatMedia(ctx, ffmpeg.ConcatMediaOptions{
		Inputs:     segments,
		OutputPath: scaled,
		Width:      320,
		Height:     180,
	})
	if err != nil {
		t.Fatalf("ConcatMedia rescale: %v", err)
	}
	if plan.Action != "transcode" {
		t.Fatalf("rescale plan = %q, want transcode", plan.Action)
	}
	info, err := svc.ProbeFile(ctx, scaled)
	if err != nil {
		t.Fatalf("probe scaled: %v", err)
	}
	if info.Width != 320 || info.Height != 180 {
		t.Fatalf("scaled size = %dx%d, want 320x180", info.Width, info.Height)
	}
}

func newIntegrationService(t *testing.T, ctx context.Context) *ffmpeg.Service {
	t.Helper()
	requireFFmpeg(t)
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{})
	if err != nil {
		t.Fatalf("ffmpeg.New: %v", err)
	}
	return svc
}

func listSegments(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(matches)
	for _, m := range matches {
		if _, err := os.Stat(m); err != nil {
			t.Fatal(err)
		}
	}
	return matches
}

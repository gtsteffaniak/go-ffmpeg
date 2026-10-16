//go:build integration

package ffmpeg_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func requireFFmpeg(t *testing.T) (ffmpegBin, ffprobeBin string) {
	t.Helper()
	if p := os.Getenv("GOFFMPEG_FFMPEG_PATH"); p != "" {
		return p, os.Getenv("GOFFMPEG_FFPROBE_PATH")
	}
	var err error
	ffmpegBin, err = exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatalf("ffmpeg is required: install ffmpeg or set GOFFMPEG_FFMPEG_PATH (%v)", err)
	}
	ffprobeBin, err = exec.LookPath("ffprobe")
	if err != nil {
		t.Fatalf("ffprobe is required: install ffprobe or set GOFFMPEG_FFPROBE_PATH (%v)", err)
	}
	return ffmpegBin, ffprobeBin
}

func TestIntegrationDetect(t *testing.T) {
	ffmpegBin, ffprobeBin := requireFFmpeg(t)
	skipHW := os.Getenv("GOFFMPEG_SKIP_HW") == "1"
	svc, err := ffmpeg.New(context.Background(), ffmpeg.Config{
		FFmpegPath:  ffmpegBin,
		FFprobePath: ffprobeBin,
		SkipHWTests: skipHW,
	})
	if err != nil {
		t.Fatal(err)
	}
	caps := svc.Capabilities()
	if caps.FFmpegVersion == "" {
		t.Fatal("missing version")
	}
	t.Log(caps.ReportString())
}

func integrationSampleMP4(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("GOFFMPEG_SAMPLE_MP4"); p != "" {
		return p
	}
	for _, rel := range []string{
		"test/data/Big_Buck_Bunny_1080_10s_2MB.mp4",
		filepath.Join("..", "test", "data", "Big_Buck_Bunny_1080_10s_2MB.mp4"),
	} {
		if st, err := os.Stat(rel); err == nil && !st.IsDir() {
			abs, err := filepath.Abs(rel)
			if err == nil {
				return abs
			}
			return rel
		}
	}
	t.Fatalf("sample not found: set GOFFMPEG_SAMPLE_MP4 or add test/data/Big_Buck_Bunny_1080_10s_2MB.mp4")
	return ""
}

func TestIntegrationMediaDuration(t *testing.T) {
	sample := integrationSampleMP4(t)
	ffmpegBin, ffprobeBin := requireFFmpeg(t)
	svc, err := ffmpeg.New(context.Background(), ffmpeg.Config{
		FFmpegPath:  ffmpegBin,
		FFprobePath: ffprobeBin,
		SkipHWTests: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	dur, err := svc.GetMediaDuration(context.Background(), sample)
	if err != nil {
		t.Fatal(err)
	}
	// Big_Buck_Bunny_1080_10s_2MB.mp4 is ~10s; allow encoder/probe rounding.
	if dur < 9.0 || dur > 11.0 {
		t.Fatalf("duration = %.3fs, want ~10s ±1s", dur)
	}
}

func TestIntegrationScreenshot(t *testing.T) {
	sample := integrationSampleMP4(t)
	ffmpegBin, ffprobeBin := requireFFmpeg(t)
	svc, err := ffmpeg.New(context.Background(), ffmpeg.Config{
		FFmpegPath:  ffmpegBin,
		FFprobePath: ffprobeBin,
		SkipHWTests: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "frame.jpg")
	err = svc.Screenshot(context.Background(), ffmpeg.ScreenshotOptions{
		Input:      ffmpeg.InputSource{URL: sample, StreamType: ffmpeg.StreamFile},
		OutputPath: out,
		Quality:    5,
		Timeout:    30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(out)
	if err != nil || st.Size() == 0 {
		t.Fatal("screenshot empty")
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 3 || data[0] != 0xFF || data[1] != 0xD8 || data[2] != 0xFF {
		t.Fatalf("screenshot is not a JPEG (magic % x)", data[:min(3, len(data))])
	}
	w, h, err := svc.GetImageDimensions(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if w <= 0 || h <= 0 {
		t.Fatalf("dimensions = %dx%d", w, h)
	}
}

func TestIntegrationHLSSegmentMPEGTS(t *testing.T) {
	sample := integrationSampleMP4(t)
	ffmpegBin, ffprobeBin := requireFFmpeg(t)
	ctx := context.Background()
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{
		FFmpegPath:  ffmpegBin,
		FFprobePath: ffprobeBin,
		SkipHWTests: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	info, err := svc.ProbeFile(ctx, sample)
	if err != nil {
		t.Fatal(err)
	}
	buildInput := ffmpeg.BuildHLSSegmentBuildInput(info, ffmpeg.HLSPipelineOptions{ForceVideoTranscode: true})
	params := ffmpeg.BuildHLSSegmentParamsFast(buildInput, ffmpeg.DefaultOnDemandHLSDefaults())
	if params.Remux || params.VideoCopy {
		t.Fatalf("params = %+v, want full transcode", params)
	}

	dur, err := svc.GetMediaDuration(ctx, sample)
	if err != nil {
		t.Fatal(err)
	}
	starts, durations := ffmpeg.BuildHLSSegmentTimeline(dur, nil, ffmpeg.DefaultHLSSegmentDurationSec)
	midIndex := len(starts) / 2
	if midIndex < 1 {
		midIndex = 1
	}

	for _, idx := range []int{0, midIndex} {
		idx := idx
		t.Run(fmt.Sprintf("segment_%d", idx), func(t *testing.T) {
			opts := ffmpeg.BuildHLSSegmentOptions(sample, idx, params, starts, durations, false, nil, ffmpeg.DefaultHLSSegmentDurationSec)
			data, err := svc.HLSSegment(ctx, opts)
			if err != nil {
				t.Fatalf("HLSSegment index %d start %.3fs: %v", idx, opts.StartSec, err)
			}
			assertProbeableMPEGTSSegment(t, ffprobeBin, data, idx)
		})
	}
}

func assertProbeableMPEGTSSegment(t *testing.T, ffprobeBin string, data []byte, index int) {
	t.Helper()
	if len(data) < 4096 {
		t.Fatalf("segment %d: too small (%d bytes)", index, len(data))
	}
	if data[0] != 0x47 {
		t.Fatalf("segment %d: missing MPEG-TS sync byte (got 0x%02x)", index, data[0])
	}
	path := filepath.Join(t.TempDir(), fmt.Sprintf("seg%d.ts", index))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ffprobeBin, "-v", "error", "-show_entries", "format=format_name", "-of", "default=nw=1:nk=1", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("segment %d ffprobe: %v\n%s", index, err, out)
	}
	format := strings.TrimSpace(string(out))
	if format == "" || !strings.Contains(format, "mpegts") {
		t.Fatalf("segment %d: ffprobe format_name = %q, want mpegts", index, format)
	}
}

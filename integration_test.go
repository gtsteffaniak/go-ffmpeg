//go:build integration

package ffmpeg_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
		t.Skip("ffmpeg not in PATH; set GOFFMPEG_FFMPEG_PATH")
	}
	ffprobeBin, err = exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not in PATH")
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

func TestIntegrationMediaDuration(t *testing.T) {
	sample := os.Getenv("GOFFMPEG_SAMPLE_MP4")
	if sample == "" {
		t.Skip("set GOFFMPEG_SAMPLE_MP4 to run")
	}
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
	if dur <= 0 {
		t.Fatalf("duration = %v", dur)
	}
}

func TestIntegrationScreenshot(t *testing.T) {
	sample := os.Getenv("GOFFMPEG_SAMPLE_MP4")
	if sample == "" {
		t.Skip("set GOFFMPEG_SAMPLE_MP4 to run")
	}
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
}

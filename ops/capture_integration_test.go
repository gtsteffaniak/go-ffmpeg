//go:build integration

package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestScreenshotMPEGTSLightProbe(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	input := filepath.Join(t.TempDir(), "capture.ts")
	generateIntegrationMPEGTS(t, ffmpegPath, input, 12)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	dur, err := probe.GetMediaDuration(ctx, runner, input)
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "frame.jpg")
	if err := Screenshot(ctx, runner, ScreenshotOptions{
		Input:       InputSource{URL: input, StreamType: probe.StreamFile},
		OutputPath:  out,
		SeekSec:     dur / 2,
		DurationSec: dur,
		Quality:     85,
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 3 || data[0] != 0xFF || data[1] != 0xD8 {
		t.Fatalf("not a JPEG: % x", data[:min(4, len(data))])
	}
}

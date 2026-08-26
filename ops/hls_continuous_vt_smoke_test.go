//go:build darwin

package ops

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestHLSContinuousTranscodeVTSmoke(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin only")
	}
	if testing.Short() {
		t.Skip("smoke test")
	}

	runner := &ffexec.Runner{FFmpegPath: "/opt/homebrew/bin/ffmpeg", FFprobePath: "/opt/homebrew/bin/ffprobe"}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	dir := t.TempDir()
	input := filepath.Join(dir, "input.mkv")
	outDir := filepath.Join(dir, "job")
	if err := os.MkdirAll(filepath.Join(outDir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	gen := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=10:size=1920x1080:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", input,
	}
	if _, err := runner.RunFFmpeg(ctx, gen...); err != nil {
		t.Fatalf("generate input: %v", err)
	}

	opts := HLSContinuousOptions{
		Input:      InputSource{URL: input, StreamType: probe.StreamFile},
		OutputDir:  outDir,
		SegmentSec: 4,
		MaxHeight:  720,
		Profile:    encode.DefaultHLSVideoProfile(720),
		Decode:     encode.HLSDecodeProfileForOnDemand(probe.StreamInfo{VideoCodec: "h264", HasVideo: true}),
	}
	job, err := StartHLSContinuous(ctx, runner, caps, opts)
	if err != nil {
		t.Fatalf("StartHLSContinuous: %v", err)
	}
	defer func() {
		job.Cancel()
		_ = job.Wait()
	}()

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if continuousSegmentReady(outDir, 0, false, opts) {
			if _, err := os.Stat(filepath.Join(outDir, "init.m4s")); err == nil {
				job.Cancel()
				_ = job.Wait()
				args, _ := buildHLSContinuousArgs(runner, caps, opts)
				if strings.Contains(strings.Join(args, " "), "-hwaccel videotoolbox") {
					t.Log("segment 0 ready with VT hw decode")
					return
				}
				t.Log("segment 0 ready (software encode path)")
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	args, _ := buildHLSContinuousArgs(runner, caps, opts)
	t.Fatalf("timeout waiting for segment 0; ffmpeg args: %s", strings.Join(args, " "))
}

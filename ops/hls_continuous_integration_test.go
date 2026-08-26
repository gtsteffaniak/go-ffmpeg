//go:build integration

package ops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestContinuousSoftwareTranscodeFullFile(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	input := filepath.Join(t.TempDir(), "input.mkv")
	generateIntegrationH264MKV(t, ffmpegPath, input, 32)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{SkipHWTests: true})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "job")
	if err := os.MkdirAll(filepath.Join(outDir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: input, StreamType: probe.StreamFile},
		OutputDir:  outDir,
		SegmentSec: 4,
		MaxHeight:  720,
		Profile:    encode.DefaultHLSVideoProfile(720),
		Decode:     encode.VideoDecodeProfile{Codec: capabilities.CodecH264, Accel: capabilities.AccelNone},
	}
	job, err := StartHLSContinuous(ctx, runner, caps, opts)
	if err != nil {
		t.Fatalf("StartHLSContinuous: %v", err)
	}
	if err := job.Wait(); err != nil {
		t.Fatalf("job wait: %v", err)
	}
	if err := AlignContinuousHLSSegments(outDir, opts); err != nil {
		t.Fatalf("AlignContinuousHLSSegments: %v", err)
	}

	result, err := ValidateContinuousHLSOutput(outDir, opts, true, 0.15)
	if err != nil {
		t.Fatalf("ValidateContinuousHLSOutput: %v", err)
	}
	if result.SegmentCount < 7 {
		t.Fatalf("segment count = %d, want >= 7 for 32s @ 4s", result.SegmentCount)
	}
	if !result.HasEndList {
		t.Fatal("missing #EXT-X-ENDLIST")
	}
	if len(result.Issues) > 0 {
		t.Fatalf("validation issues: %+v", result.Issues)
	}
}

func integrationFFmpegPaths(t *testing.T) (ffmpeg, ffprobe string) {
	t.Helper()
	ffmpeg = os.Getenv("GOFFMPEG_FFMPEG_PATH")
	ffprobe = os.Getenv("GOFFMPEG_FFPROBE_PATH")
	if ffmpeg == "" {
		var err error
		ffmpeg, err = exec.LookPath("ffmpeg")
		if err != nil {
			t.Fatalf("ffmpeg is required: %v", err)
		}
	}
	if ffprobe == "" {
		var err error
		ffprobe, err = exec.LookPath("ffprobe")
		if err != nil {
			t.Fatalf("ffprobe is required: %v", err)
		}
	}
	return ffmpeg, ffprobe
}

func generateIntegrationH264MKV(t *testing.T, ffmpegPath, outPath string, durationSec int) {
	t.Helper()
	cmd := exec.Command(ffmpegPath,
		"-hide_banner", "-y",
		"-f", "lavfi", "-i", "testsrc=duration="+strconv.Itoa(durationSec)+":size=1280x720:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440:duration="+strconv.Itoa(durationSec),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-g", "48", "-keyint_min", "48",
		"-preset", "veryfast", "-crf", "23",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate fixture: %v\n%s", err, out)
	}
}

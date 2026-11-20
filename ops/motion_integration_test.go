package ops

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func requireFFmpeg(t *testing.T) (string, string) {
	t.Helper()
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	fp, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	return ff, fp
}

func TestDetectMotionIntegration(t *testing.T) {
	ff, fp := requireFFmpeg(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "motion.mp4")

	// 3s static gray, 3s moving testsrc, 3s static gray.
	cmd := exec.Command(ff, "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=gray:s=320x240:d=3:r=10",
		"-f", "lavfi", "-i", "testsrc=s=320x240:d=3:r=10",
		"-f", "lavfi", "-i", "color=c=gray:s=320x240:d=3:r=10",
		"-filter_complex", "[0:v][1:v][2:v]concat=n=3:v=1[v]",
		"-map", "[v]", "-pix_fmt", "yuv420p", "-c:v", "libx264", "-g", "5", out, "-y")
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture encode failed: %v: %s", err, combined)
	}

	runner := &ffexec.Runner{FFmpegPath: ff, FFprobePath: fp}
	result, err := DetectMotion(context.Background(), runner, &capabilities.Capabilities{},
		MotionDetectOptions{
			Input:     InputSource{URL: out, StreamType: probe.StreamFile},
			SampleFPS: 2,
			Threshold: 2,
			Timeout:   time.Minute,
		})
	if err != nil {
		t.Fatalf("DetectMotion failed: %v", err)
	}
	t.Logf("samples=%d events=%d peak=%.2f", result.Samples, len(result.Events), result.PeakScore)
	if result.Samples == 0 {
		t.Fatal("expected samples from motion analysis")
	}
	if len(result.Events) == 0 {
		t.Fatalf("expected at least one motion event, peak=%f", result.PeakScore)
	}
	first := result.Events[0]
	t.Logf("first event %.2f-%.2f peak=%.2f", first.StartSec, first.EndSec, first.PeakScore)
	// Motion starts at ~3s; allow generous slack for keyframe/timestamp handling.
	if first.StartSec < 2.0 || first.StartSec > 4.5 {
		t.Fatalf("first event should start near 3s, got %.2f", first.StartSec)
	}
}

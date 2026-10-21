//go:build integration

package ops

import (
	"context"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

func TestProbeVideoKeyframeTimesContainerMatrix(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx := context.Background()

	base := filepath.Join(t.TempDir(), "base.mp4")
	generateIntegrationH264MP4(t, ffmpegPath, base, 10)
	refTimes, err := ProbeVideoKeyframeTimes(ctx, runner, base)
	if err != nil {
		t.Fatalf("reference mp4 keyframes: %v", err)
	}
	refNorm := normalizeKeyframeTimeline(refTimes)

	fixtures := []struct {
		name string
		gen  func(t *testing.T, dir string) string
	}{
		{
			name: "mpegts",
			gen: func(t *testing.T, dir string) string {
				t.Helper()
				out := filepath.Join(dir, "clip.ts")
				runFFmpeg(t, ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
					"-i", base, "-c", "copy", "-f", "mpegts", out)
				return out
			},
		},
		{
			name: "m2ts",
			gen: func(t *testing.T, dir string) string {
				t.Helper()
				ts := filepath.Join(dir, "clip.ts")
				runFFmpeg(t, ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
					"-i", base, "-c", "copy", "-f", "mpegts", ts)
				out := filepath.Join(dir, "clip.m2ts")
				if err := exec.Command("cp", ts, out).Run(); err != nil {
					t.Fatal(err)
				}
				return out
			},
		},
		{
			name: "avi-h264-bframes",
			gen: func(t *testing.T, dir string) string {
				t.Helper()
				out := filepath.Join(dir, "clip.avi")
				runFFmpeg(t, ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
					"-i", base, "-c:v", "libx264", "-bf", "2", "-f", "avi", out)
				return out
			},
		},
		{
			name: "avi-mpeg4",
			gen: func(t *testing.T, dir string) string {
				t.Helper()
				out := filepath.Join(dir, "clip.avi")
				runFFmpeg(t, ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
					"-i", base, "-c:v", "mpeg4", "-f", "avi", out)
				return out
			},
		},
		{
			name: "raw-h264",
			gen: func(t *testing.T, dir string) string {
				t.Helper()
				out := filepath.Join(dir, "clip.h264")
				runFFmpeg(t, ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
					"-i", base, "-c:v", "copy", "-bsf:v", "h264_mp4toannexb", "-f", "h264", out)
				return out
			},
		},
	}

	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			path := fx.gen(t, dir)
			got, err := ProbeVideoKeyframeTimes(ctx, runner, path)
			if err != nil {
				t.Fatalf("packet probe: %v", err)
			}
			if len(got) == 0 {
				t.Fatal("expected non-empty keyframe timeline")
			}
			gotNorm := normalizeKeyframeTimeline(got)
			frameTimes, frameErr := probeVideoKeyframeTimesByFrame(ctx, runner, path)
			if frameErr == nil && len(frameTimes) > 0 {
				assertKeyframeTimelinesMatch(t, got, frameTimes)
			} else {
				assertKeyframeTimelinesMatch(t, gotNorm, refNorm)
			}
		})
	}
}

func normalizeKeyframeTimeline(times []float64) []float64 {
	if len(times) == 0 {
		return nil
	}
	base := times[0]
	out := make([]float64, len(times))
	for i, t := range times {
		out[i] = t - base
	}
	return out
}

func assertKeyframeTimelinesMatch(t *testing.T, got, ref []float64) {
	t.Helper()
	if len(ref) == 0 {
		t.Fatal("reference timeline empty")
	}
	// One frame at 24fps ≈ 42ms; allow a full frame of slack between probe methods.
	const maxSlack = 1.0 / 23.976
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("times not strictly increasing at %d: %.6f <= %.6f", i, got[i], got[i-1])
		}
	}
	limit := len(got)
	if len(ref) < limit {
		limit = len(ref)
	}
	for i := 0; i < limit; i++ {
		if math.Abs(got[i]-ref[i]) > maxSlack {
			t.Fatalf("index %d: got=%.6f ref=%.6f (slack %.3f)", i, got[i], ref[i], maxSlack)
		}
	}
}

func keyframeNear(times []float64, target, slack float64) bool {
	for _, t := range times {
		if math.Abs(t-target) <= slack {
			return true
		}
	}
	return false
}

func generateIntegrationH264MP4(t *testing.T, ffmpegPath, outPath string, durationSec int) {
	t.Helper()
	runFFmpeg(t, ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration="+strconv.Itoa(durationSec)+":size=320x240:rate=24",
		"-c:v", "libx264", "-g", "24", "-keyint_min", "24", "-pix_fmt", "yuv420p",
		outPath)
}

func runFFmpeg(t testing.TB, ffmpegPath string, args ...string) {
	t.Helper()
	cmd := exec.Command(ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg %v: %v\n%s", args, err, out)
	}
}

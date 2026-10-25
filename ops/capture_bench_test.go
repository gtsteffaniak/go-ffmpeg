package ops

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// CaptureTiming splits software frame capture into probe vs ffmpeg (seek+decode).
type CaptureTiming struct {
	ProbeSec  float64
	FFmpegSec float64
	TotalSec  float64
	SeekSec   float64
	Accuracy  CaptureAccuracy
	SkipProbe bool
}

func measureSoftwareCapture(ctx context.Context, runner *ffexec.Runner, input string, seekSec float64, accuracy CaptureAccuracy, skipProbe bool) (CaptureTiming, error) {
	var timing CaptureTiming
	timing.SeekSec = seekSec
	timing.Accuracy = accuracy
	timing.SkipProbe = skipProbe

	probeStart := time.Now()
	dur, err := probe.GetMediaDuration(ctx, runner, input)
	if err != nil {
		return timing, err
	}
	timing.ProbeSec = time.Since(probeStart).Seconds()

	src := InputSource{URL: input, StreamType: probe.StreamFile}
	args := BuildFrameCaptureArgs(runner, src, nil, FrameCaptureArgs{
		SeekSec:      seekSec,
		Accuracy:     accuracy,
		SkipProbe:    skipProbe || dur > 0,
		KeyframeOnly: accuracy == CaptureFast,
		MJPEG:        true,
		QScale:       "5",
	})

	ffStart := time.Now()
	if err := runFrameCaptureToWriter(ctx, runner, args, io.Discard); err != nil {
		return timing, err
	}
	timing.FFmpegSec = time.Since(ffStart).Seconds()
	timing.TotalSec = timing.ProbeSec + timing.FFmpegSec
	return timing, nil
}

func integrationSample(tb testing.TB) (string, *ffexec.Runner) {
	tb.Helper()
	path := os.Getenv("GOFFMPEG_SAMPLE_MP4")
	if path == "" {
		path = filepath.Join("..", "test", "data", "Big_Buck_Bunny_1080_10s_2MB.mp4")
	}
	if _, err := os.Stat(path); err != nil {
		tb.Skip("sample mp4 not found")
	}
	ffmpegBin := os.Getenv("GOFFMPEG_FFMPEG_PATH")
	ffprobeBin := os.Getenv("GOFFMPEG_FFPROBE_PATH")
	ffmpegPath, ffprobePath, err := ffexec.ResolvePair(ffmpegBin, ffprobeBin)
	if err != nil {
		tb.Skip(err.Error())
	}
	return path, &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
}

func TestCaptureTimingReport(t *testing.T) {
	input, runner := integrationSample(t)
	ctx := context.Background()

	fastLate, err := measureSoftwareCapture(ctx, runner, input, 7.0, CaptureFast, true)
	if err != nil {
		t.Fatal(err)
	}
	exactLate, err := measureSoftwareCapture(ctx, runner, input, 7.0, CaptureExact, true)
	if err != nil {
		t.Fatal(err)
	}
	fastEarly, err := measureSoftwareCapture(ctx, runner, input, 0.5, CaptureFast, true)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("capture timing (software, skip heavy probe when duration known):")
	t.Logf("  seek=7s fast:   probe=%.3fs ffmpeg=%.3fs total=%.3fs", fastLate.ProbeSec, fastLate.FFmpegSec, fastLate.TotalSec)
	t.Logf("  seek=7s exact:  probe=%.3fs ffmpeg=%.3fs total=%.3fs", exactLate.ProbeSec, exactLate.FFmpegSec, exactLate.TotalSec)
	t.Logf("  seek=0.5s fast: probe=%.3fs ffmpeg=%.3fs total=%.3fs", fastEarly.ProbeSec, fastEarly.FFmpegSec, fastEarly.TotalSec)
	if fastLate.FFmpegSec > exactLate.FFmpegSec*1.5 {
		t.Logf("note: fast keyframe capture was not faster than exact at this seek; GOP layout may dominate")
	}
}

func BenchmarkSoftwareFrameCapture(b *testing.B) {
	input, runner := integrationSample(b)
	ctx := context.Background()
	b.Run("fast_seek7", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := measureSoftwareCapture(ctx, runner, input, 7, CaptureFast, true)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("exact_seek7", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := measureSoftwareCapture(ctx, runner, input, 7, CaptureExact, true)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

//go:build darwin

package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestSuccessionBalancedTranscodeSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Succession smoke in -short mode")
	}
	input := os.Getenv("HLS_SMOKE_INPUT")
	if input == "" {
		t.Skip("set HLS_SMOKE_INPUT to run Succession balanced transcode smoke")
	}
	if _, err := os.Stat(input); err != nil {
		t.Skipf("HLS_SMOKE_INPUT not readable: %v", err)
	}

	runner := &ffexec.Runner{FFmpegPath: "/opt/homebrew/bin/ffmpeg", FFprobePath: "/opt/homebrew/bin/ffprobe"}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "job")
	if err := os.MkdirAll(filepath.Join(outDir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	decode := encode.HLSDecodeProfileForOnDemand(probe.StreamInfo{VideoCodec: "h264", HasVideo: true})
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: input, StreamType: probe.StreamFile},
		OutputDir:  outDir,
		SegmentSec: 4,
		MaxHeight:  720,
		Profile:    encode.DefaultHLSVideoProfile(720),
		Decode:     decode,
	}

	const wantThrough = 1
	job, stopAttempt, err := startContinuousWithVTFallback(ctx, t, runner, caps, outDir, opts, wantThrough)
	if err != nil {
		t.Fatal(err)
	}
	defer stopAttempt()
	defer func() {
		job.Cancel()
		_ = job.Wait()
	}()
}

func startContinuousWithVTFallback(ctx context.Context, t *testing.T, runner *ffexec.Runner, caps *capabilities.Capabilities, outDir string, opts HLSContinuousOptions, wantThrough int) (*HLSContinuousJob, context.CancelFunc, error) {
	t.Helper()
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, func() {}, err
		}
		attemptCtx, attemptCancel := context.WithTimeout(ctx, 3*time.Minute)
		job, err := StartHLSContinuous(attemptCtx, runner, caps, opts)
		if err != nil {
			attemptCancel()
			return nil, func() {}, fmt.Errorf("StartHLSContinuous: %w", err)
		}
		if waitContinuousThrough(attemptCtx, outDir, opts, job, wantThrough) {
			return job, attemptCancel, nil
		}
		vtFallback := job.VTDecodeUnreliable()
		job.Cancel()
		_ = job.Wait()
		attemptCancel()
		if !vtFallback || attempt > 0 {
			return nil, func() {}, fmt.Errorf("timeout waiting for segments 0-%d (vtFallback=%t)", wantThrough, vtFallback)
		}
		t.Log("VT decode unreliable; restarting with CPU decode")
		opts.Decode = encode.SoftwareDecodeProfile(opts.Decode)
		_ = os.RemoveAll(filepath.Join(outDir, "seg"))
		_ = os.Remove(filepath.Join(outDir, "init.m4s"))
		_ = os.Remove(filepath.Join(outDir, "ffmpeg.m3u8"))
		if err := os.MkdirAll(filepath.Join(outDir, "seg"), 0o755); err != nil {
			return nil, func() {}, err
		}
	}
	return nil, func() {}, fmt.Errorf("unreachable")
}

func waitContinuousThrough(ctx context.Context, outDir string, opts HLSContinuousOptions, job *HLSContinuousJob, wantThrough int) bool {
	for {
		ready := true
		for i := 0; i <= wantThrough; i++ {
			if !continuousSegmentReady(outDir, i, false, opts) {
				ready = false
				break
			}
			if _, err := os.Stat(filepath.Join(outDir, "seg", fmt.Sprintf("%05d.m4s.ready", i))); err != nil {
				ready = false
				break
			}
		}
		if ready {
			if _, err := os.Stat(filepath.Join(outDir, "init.m4s")); err == nil {
				return true
			}
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(200 * time.Millisecond):
		}
	}
}

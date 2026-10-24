package ops

import (
	"strings"
	"testing"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestBuildFrameCaptureArgsFastSeek(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{}
	input := InputSource{URL: "in.mp4", StreamType: probe.StreamFile}
	args := BuildFrameCaptureArgs(runner, input, nil, FrameCaptureArgs{
		SeekSec:      5.5,
		Accuracy:     CaptureFast,
		SkipProbe:    true,
		KeyframeOnly: true,
		ScaleFilter:  "scale=640:360",
		QScale:       "5",
		MJPEG:        true,
	})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-ss 5.500") {
		t.Fatalf("missing input seek: %s", joined)
	}
	ssIdx := indexOf(args, "-ss")
	iIdx := indexOf(args, "-i")
	if ssIdx < 0 || iIdx < 0 || ssIdx > iIdx {
		t.Fatalf("expected -ss before -i: %v", args)
	}
	if !sliceContains(args, "-noaccurate_seek") {
		t.Fatalf("expected -noaccurate_seek: %v", args)
	}
	if !containsArg(args, "-skip_frame", "nokey") {
		t.Fatalf("expected keyframe-only decode: %v", args)
	}
	if !containsArg(args, "-sws_flags", captureSWSFlags) {
		t.Fatalf("expected fast bilinear scaler: %v", args)
	}
	if !strings.HasSuffix(joined, "image2pipe -") {
		t.Fatalf("expected image2pipe stdout: %s", joined)
	}
	if !containsArg(args, "-probesize", CaptureDefaultLightProbeSize) {
		t.Fatalf("expected default light probesize: %v", args)
	}
	if !containsArg(args, "-analyzeduration", CaptureDefaultLightAnalyzeDuration) {
		t.Fatalf("expected default light analyzeduration: %v", args)
	}
}

func TestBuildFrameCaptureArgsLightProbeOverride(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{}
	input := InputSource{URL: "in.ts", StreamType: probe.StreamFile}
	args := BuildFrameCaptureArgs(runner, input, nil, FrameCaptureArgs{
		SkipProbe: true,
		LightProbe: &CaptureLightProbe{
			ProbeSize:       "5242880",
			AnalyzeDuration: "2000000",
		},
		MJPEG: true,
	})
	if !containsArg(args, "-probesize", "5242880") {
		t.Fatalf("custom probesize: %v", args)
	}
	if !containsArg(args, "-analyzeduration", "2000000") {
		t.Fatalf("custom analyzeduration: %v", args)
	}
}

func TestBuildFrameCaptureArgsNoLightProbeWhenFullProbe(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{}
	input := InputSource{URL: "in.mp4", StreamType: probe.StreamFile}
	args := BuildFrameCaptureArgs(runner, input, nil, FrameCaptureArgs{
		SkipProbe: false,
		MJPEG:     true,
	})
	if sliceContains(args, "-probesize") {
		t.Fatalf("full probe mode should not set probesize: %v", args)
	}
}

func TestBuildFrameCaptureArgsExactSeek(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{}
	input := InputSource{URL: "in.mp4", StreamType: probe.StreamFile}
	args := BuildFrameCaptureArgs(runner, input, nil, FrameCaptureArgs{
		SeekSec:   2,
		Accuracy:  CaptureExact,
		SkipProbe: true,
		MJPEG:     true,
	})
	if !sliceContains(args, "-accurate_seek") {
		t.Fatalf("expected -accurate_seek: %v", args)
	}
	if containsArg(args, "-skip_frame", "nokey") {
		t.Fatalf("exact mode should not skip non-keyframes: %v", args)
	}
}

func TestResolveCaptureSeekSec(t *testing.T) {
	t.Parallel()
	if got := ResolveCaptureSeekSec(100, 0, 25); got != 25 {
		t.Fatalf("percent seek = %v", got)
	}
	if got := ResolveCaptureSeekSec(100, 120, 0); got != 100 {
		t.Fatalf("clamp seek = %v", got)
	}
}

func indexOf(args []string, s string) int {
	for i, a := range args {
		if a == s {
			return i
		}
	}
	return -1
}

func containsArg(args []string, key, val string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == val {
			return true
		}
	}
	return false
}

func sliceContains(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}

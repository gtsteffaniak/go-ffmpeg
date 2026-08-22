package ops

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

func TestBuildHLSContinuousArgsDefaultCacheFillNoReadrate(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	caps := &capabilities.Capabilities{
		FeatureFlags: capabilities.FeatureFlags{Version: capabilities.Version{8, 0, 0}, Readrate: true, ReadrateCatchup: true},
	}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: "/media/video.mp4"},
		OutputDir:  "/cache/job",
		SegmentSec: 4,
		Pacing:     HLSContinuousCacheFill,
		Remux:      true,
	}

	args, err := buildHLSContinuousArgs(runner, caps, opts)
	if err != nil {
		t.Fatalf("buildHLSContinuousArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "-readrate") {
		t.Fatalf("cache-fill should not use readrate: %s", joined)
	}
}

func TestBuildHLSContinuousArgsLivePacedReadrate(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	caps := &capabilities.Capabilities{
		FeatureFlags: capabilities.FeatureFlags{
			Version:         capabilities.Version{8, 0, 0},
			Readrate:        true,
			ReadrateCatchup: true,
		},
	}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: "/media/video.mp4"},
		OutputDir:  "/cache/job",
		SegmentSec: 4,
		Pacing:     HLSContinuousLivePaced,
		Remux:      true,
	}

	args, err := buildHLSContinuousArgs(runner, caps, opts)
	if err != nil {
		t.Fatalf("buildHLSContinuousArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-readrate 1",
		"-readrate_catchup 2",
		"-readrate_initial_burst 60",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
}

func TestBuildHLSContinuousArgsExplicitThrottleOverridesPacing(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	caps := &capabilities.Capabilities{
		FeatureFlags: capabilities.FeatureFlags{
			Version:         capabilities.Version{8, 0, 0},
			Readrate:        true,
			ReadrateCatchup: true,
		},
	}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: "/media/video.mp4"},
		OutputDir:  "/cache/job",
		SegmentSec: 4,
		Pacing:     HLSContinuousLivePaced,
		Remux:      true,
		Throttle: encode.ThrottleConfig{
			Enabled:      true,
			Rate:         1.5,
			Catchup:      3,
			InitialBurst: 24,
		},
	}

	args, err := buildHLSContinuousArgs(runner, caps, opts)
	if err != nil {
		t.Fatalf("buildHLSContinuousArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-readrate 1.5", "-readrate_catchup 3", "-readrate_initial_burst 24"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
}

func TestResolveContinuousThrottle(t *testing.T) {
	t.Parallel()
	off := resolveContinuousThrottle(HLSContinuousOptions{Pacing: HLSContinuousCacheFill})
	if off.Enabled {
		t.Fatal("cache fill should be off")
	}
	live := resolveContinuousThrottle(HLSContinuousOptions{Pacing: HLSContinuousLivePaced})
	if !live.Enabled || live.InitialBurst != encode.DefaultLivePacedBurstSec {
		t.Fatalf("live paced = %+v", live)
	}
	override := resolveContinuousThrottle(HLSContinuousOptions{
		Pacing:   HLSContinuousCacheFill,
		Throttle: encode.ThrottleConfig{Enabled: true, Rate: 2},
	})
	if override.Rate != 2 {
		t.Fatalf("explicit throttle = %+v", override)
	}
}

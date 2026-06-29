package capabilities

import (
	"context"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// DetectOptions configures capability detection.
type DetectOptions struct {
	SkipHWTests      bool
	EncoderHierarchy []AccelType
	RenderDevice     string
}

// Detect runs the full capability detection pipeline.
func Detect(ctx context.Context, runner *ffexec.Runner, opts DetectOptions) (*Capabilities, error) {
	caps := NewCapabilities()
	caps.FFmpegPath = runner.FFmpegPath
	caps.FFprobePath = runner.FFprobePath
	caps.GeneratedAt = time.Now()

	verRes, err := runner.RunFFmpeg(ctx, "-version")
	if err != nil {
		return nil, err
	}
	caps.FFmpegVersion, caps.BuildConfig = ParseVersionOutput(verRes.Stdout)
	if ver, parseErr := ParseSemver(caps.FFmpegVersion); parseErr == nil {
		caps.FeatureFlags = FeatureFlagsFromVersion(ver)
	}

	probeVerRes, err := runner.RunFFprobe(ctx, "-version")
	if err != nil {
		return nil, err
	}
	caps.FFprobeVersion = ParseFFprobeVersion(probeVerRes.Stdout)

	caps.Platform = platform.Detect()
	if opts.RenderDevice != "" {
		if caps.Platform.Details == nil {
			caps.Platform.Details = map[string]string{}
		}
		caps.Platform.Details["render_device"] = opts.RenderDevice
	}

	hierarchy := opts.EncoderHierarchy
	if len(hierarchy) == 0 {
		hierarchy = HierarchyForPlatform(caps.Platform)
	}

	PopulateEncoders(ctx, caps, runner, opts.SkipHWTests, caps.Platform)
	PopulateHWAccels(ctx, caps, runner)
	PopulateDecoders(ctx, caps, runner, opts.SkipHWTests, caps.Platform)
	PopulateFilters(ctx, caps, runner)
	PopulateProtocols(ctx, caps, runner)

	caps.BuildProfile = InferBuildProfile(caps.BuildConfig, caps.Encoders)
	BuildCodecMatrix(caps, hierarchy)

	return caps, nil
}

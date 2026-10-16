package capabilities

import (
	"context"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// DetectOptions configures capability detection.
type DetectOptions struct {
	SkipHWTests       bool
	EncoderHierarchy  []AccelType
	EncoderPreference EncoderPreference
	RenderDevice      string
	GPU               platform.GPUChoice
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

	gpuChoice := opts.GPU
	if gpuChoice.SoftwareOnly {
		if len(opts.EncoderHierarchy) == 0 {
			opts.EncoderHierarchy = []AccelType{AccelNone}
		}
		opts.SkipHWTests = true
	}
	gpuScoped := gpuChoice.Enabled && !gpuChoice.SoftwareOnly
	caps.Platform = platform.Detect()
	sel := gpuChoice
	if sel.Mode == "" {
		sel.Mode = platform.GPUModeAuto
	}
	if !sel.Enabled && !sel.SoftwareOnly {
		sel.AutoSelect = true
	}
	caps.GPUSelection = &sel
	if gpuScoped {
		caps.Platform = platform.ScopedPlatform(caps.Platform, gpuChoice)
		caps.SelectedGPU = &gpuChoice
	}
	if opts.RenderDevice != "" {
		if caps.Platform.Details == nil {
			caps.Platform.Details = map[string]string{}
		}
		caps.Platform.Details["render_device"] = opts.RenderDevice
	}

	hierarchy := ResolveEncoderHierarchy(opts.EncoderHierarchy, opts.EncoderPreference, gpuChoice, caps.Platform)
	caps.EncoderHierarchy = append([]AccelType(nil), hierarchy...)

	PopulateEncoders(ctx, caps, runner, opts.SkipHWTests, caps.Platform, hierarchy, gpuScoped)
	PopulateHWAccels(ctx, caps, runner)
	PopulateDecoders(ctx, caps, runner, opts.SkipHWTests, caps.Platform, hierarchy, gpuScoped)
	PopulateFilters(ctx, caps, runner)
	PopulateProtocols(ctx, caps, runner)

	caps.BuildProfile = InferBuildProfile(caps.BuildConfig, caps.Encoders)
	BuildCodecMatrix(caps, hierarchy)

	return caps, nil
}

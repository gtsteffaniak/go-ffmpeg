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
	GPU              platform.GPUChoice
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

	gpuScoped := opts.GPU.Enabled
	caps.Platform = platform.Detect()
	if gpuScoped {
		caps.Platform = platform.ScopedPlatform(caps.Platform, opts.GPU)
		caps.SelectedGPU = &opts.GPU
	}
	if opts.RenderDevice != "" {
		if caps.Platform.Details == nil {
			caps.Platform.Details = map[string]string{}
		}
		caps.Platform.Details["render_device"] = opts.RenderDevice
	}

	hierarchy := opts.EncoderHierarchy
	if len(hierarchy) == 0 {
		if gpuScoped {
			hierarchy = HierarchyForGPU(opts.GPU.Vendor, caps.Platform)
		}
		if len(hierarchy) == 0 {
			hierarchy = HierarchyForPlatform(caps.Platform)
		}
	}
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

package capabilities

import (
	"context"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// DetectOptions configures capability detection.
type DetectOptions struct {
	SkipHWTests        bool
	EncoderHierarchy   []AccelType
	EncoderPreference  EncoderPreference
	RenderDevice       string
	GPU                platform.GPUChoice
	DeviceChain        []platform.GPUChoice
	DeviceOrderSummary string
	DevicePreference   string
	AllowSoftware      *bool // nil = true (backward compatible)
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
	hardwarePlat := platform.Detect()
	caps.Platform = hardwarePlat
	caps.HardwarePlatform = hardwarePlat
	caps.AllowSoftware = true
	if opts.AllowSoftware != nil {
		caps.AllowSoftware = *opts.AllowSoftware
	}
	sel := gpuChoice
	if sel.Mode == "" {
		sel.Mode = platform.GPUModeAuto
	}
	if !sel.Enabled && !sel.SoftwareOnly {
		sel.AutoSelect = true
	}
	caps.GPUSelection = &sel
	caps.DeviceChain = append([]platform.GPUChoice(nil), opts.DeviceChain...)
	caps.DeviceOrderSummary = opts.DeviceOrderSummary
	caps.DevicePreference = opts.DevicePreference
	if len(caps.DeviceChain) == 0 && gpuChoice.Enabled && !gpuChoice.SoftwareOnly {
		caps.DeviceChain = []platform.GPUChoice{gpuChoice}
	}
	if gpuChoice.Pinning.NVENC != "" || gpuChoice.Pinning.AMF != "" {
		caps.Pinning = gpuChoice.Pinning
	}
	if gpuScoped {
		caps.SelectedGPU = &gpuChoice
	}
	if opts.RenderDevice != "" {
		gpuChoice.RenderDevice = opts.RenderDevice
	}

	hierarchyGPU := gpuChoice
	if len(opts.DeviceChain) > 1 {
		hierarchyGPU = platform.GPUChoice{AutoSelect: true}
	}
	hierarchy := ResolveEncoderHierarchy(opts.EncoderHierarchy, opts.EncoderPreference, hierarchyGPU, hardwarePlat)
	caps.EncoderHierarchy = append([]AccelType(nil), hierarchy...)

	PopulateEncoders(ctx, caps, runner, opts.SkipHWTests, hardwarePlat, hierarchy, gpuScoped)
	PopulateHWAccels(ctx, caps, runner)
	PopulateDecoders(ctx, caps, runner, opts.SkipHWTests, hardwarePlat, hierarchy, gpuScoped)
	PopulateFilters(ctx, caps, runner)
	PopulateProtocols(ctx, caps, runner)

	caps.BuildProfile = InferBuildProfile(caps.BuildConfig, caps.Encoders)
	BuildCodecMatrix(caps, hierarchy)
	applyPiPlatformProfile(caps)
	ApplyDeviceSelection(caps, hierarchy, opts.DeviceChain, gpuChoice)

	return caps, nil
}

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
	VulkanEncoder      VulkanEncoderMode
}

// Detect runs the full capability detection pipeline.
func Detect(ctx context.Context, runner *ffexec.Runner, opts DetectOptions) (*Capabilities, error) {
	caps := NewCapabilities()
	caps.FFmpegPath = runner.FFmpegPath
	caps.FFprobePath = runner.FFprobePath
	caps.GeneratedAt = time.Now()
	caps.FFmpegBinary = ProbeFFmpegBinary(runner.FFmpegPath)
	caps.DetectionHost = NewDetectionHost()

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
	if gpuChoice.SoftwareOnly && len(opts.EncoderHierarchy) == 0 {
		// Software selection only steers which encoder is preferred. Hardware
		// backends are still smoke-tested (unless SkipHWTests is set) so hosts
		// can list what the machine could do and switch devices at runtime.
		opts.EncoderHierarchy = []AccelType{AccelNone}
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
	if nonSoftwareChainDevices(opts.DeviceChain) > 1 {
		// Multiple real devices in the chain may resolve different codecs to
		// different GPUs; a trailing software fallback entry must not count.
		hierarchyGPU = platform.GPUChoice{AutoSelect: true}
	}
	hierarchy := ResolveEncoderHierarchy(opts.EncoderHierarchy, opts.EncoderPreference, hierarchyGPU, hardwarePlat)
	hierarchy = MergeVulkanHierarchy(hierarchy, opts.EncoderHierarchy, opts.VulkanEncoder)
	caps.VulkanEncoderRequested = hierarchyIncludesAccel(hierarchy, AccelVulkan)
	caps.EncoderHierarchy = append([]AccelType(nil), hierarchy...)

	PopulateEncoders(ctx, caps, runner, opts.SkipHWTests, hardwarePlat, hierarchy, gpuScoped)
	PopulateHWAccels(ctx, caps, runner)
	PopulateDecoders(ctx, caps, runner, opts.SkipHWTests, hardwarePlat, hierarchy, gpuScoped)
	PopulateFilters(ctx, caps, runner)
	PopulateProtocols(ctx, caps, runner)

	caps.BuildProfile = InferBuildProfile(caps.BuildConfig, caps.Encoders)
	BuildCodecMatrix(caps, hierarchy)
	applyPiPlatformProfile(caps)
	if err := ApplyDeviceSelection(caps, hierarchy, opts.DeviceChain, gpuChoice); err != nil {
		return nil, err
	}
	if len(opts.EncoderHierarchy) == 0 {
		// The resolved (non-explicit) hierarchy reports what this hardware can
		// actually run; selection already gates on availability so this only
		// scopes the stored/displayed order.
		caps.EncoderHierarchy = usableHierarchy(hierarchy, caps)
	}

	return caps, nil
}

// nonSoftwareChainDevices counts real GPU devices in a resolved device chain,
// ignoring the trailing software fallback entry.
func nonSoftwareChainDevices(chain []platform.GPUChoice) int {
	n := 0
	for _, c := range chain {
		if !c.SoftwareOnly {
			n++
		}
	}
	return n
}

// usableHierarchy prunes a resolved hierarchy to backends that passed encode
// detection for at least one codec; falls back to software so the report never
// advertises a backend the hardware cannot run.
func usableHierarchy(hierarchy []AccelType, caps *Capabilities) []AccelType {
	codecs := []VideoCodec{CodecH264, CodecHEVC, CodecAV1, CodecVP9}
	out := make([]AccelType, 0, len(hierarchy))
	for _, accel := range hierarchy {
		if accel == AccelNone {
			out = append(out, accel)
			continue
		}
		for _, codec := range codecs {
			if caps.CodecEncodeAvailable(codec, accel) {
				out = append(out, accel)
				break
			}
		}
	}
	if len(out) == 0 {
		return []AccelType{AccelNone}
	}
	return out
}

package ffmpeg

import (
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/concurrency"
)

// Config configures a Service instance.
type Config struct {
	// FFmpegPath is a directory containing ffmpeg or the full path to the binary.
	FFmpegPath string

	// FFprobePath is a directory containing ffprobe or the full path to the binary.
	// When empty, ffprobe is resolved as a sibling of the ffmpeg binary.
	FFprobePath string

	// DetectOnInit runs capability detection during New. Defaults to true when nil.
	DetectOnInit *bool

	// DetectTimeout limits how long Detect may run. Default 60s.
	DetectTimeout time.Duration

	// Concurrency configures per-tier subprocess limits (probe, decode, encode).
	// Zero fields use defaults in withDefaults().
	Concurrency Concurrency

	// MaxConcurrent is deprecated: when Concurrency tier fields are unset, maps to
	// LegacyFromMaxConcurrent (MaxDecode/MaxEncode = MaxConcurrent, higher MaxProbe).
	// Prefer Concurrency for new integrations.
	MaxConcurrent int

	// Logger receives diagnostic output. Inject any implementation of Logger
	// (for example github.com/gtsteffaniak/go-logger/logger.Logger or slog via
	// FromSlog). Defaults to slog.Default() when nil.
	Logger Logger

	// EncoderHierarchy overrides hardware acceleration preference order.
	// When nil or empty, detection uses HierarchyForPlatform (VideoToolbox on macOS, etc.).
	// To disable hardware acceleration without skipping detection, pass
	// []capabilities.AccelType{capabilities.AccelNone}: hardware encoders are
	// still detected and reported, but the resolver prefers software.
	EncoderHierarchy []capabilities.AccelType

	// EncoderPreference selects backend priority when EncoderHierarchy is unset and
	// GPU is empty. Empty or "default": VAAPI-first on Linux, QSV-first on Windows.
	// "powersave" is an alias for that default. "performance" (dgpu-first) prefers NVENC before VAAPI on Linux.
	EncoderPreference string

	// VulkanEncoder controls Vulkan video encode in the default hierarchy.
	// Empty or "off": Vulkan is only used when listed in EncoderHierarchy.
	// "fallback": append Vulkan after vendor backends (lowest HW priority).
	VulkanEncoder string

	// GPUSelector selects a single GPU when set to GPUIntegrated, GPUDiscrete, GPUDevice, etc.
	// Zero value (GPUAuto) keeps automatic device hierarchy (iGPU/SoC → dGPU → software).
	// ParseGPUSelector("") and ParseGPUSelector("auto") both map to GPUAuto.
	GPUSelector GPUSelector

	// DeviceSelection sets an explicit primary GPU and ordered fallbacks evaluated per codec.
	// When unset (zero), the library builds a default chain from DevicePreference (iGPU/SoC → dGPU → software).
	DeviceSelection DeviceSelection

	// DevicePreference selects default GPU order when DeviceSelection is unset.
	// PreferIntegrated (zero): integrated/SoC → discrete → software. PreferDiscrete reverses iGPU and dGPU.
	// EncoderPreference "performance" is a deprecated alias for PreferDiscrete (device order only).
	DevicePreference DevicePreference

	// AllowSoftwareEncoderFallback defaults to true. When false, detection returns an error
	// if no hardware path works on the selected device chain.
	AllowSoftwareEncoderFallback *bool

	// GPUStrictness controls behavior when the selected GPU is unavailable.
	// Zero (GPUFallbackOnUnavailable) falls back to auto and records the reason in the capability report.
	GPUStrictness GPUStrictness

	// GPU is deprecated: use GPUSelector. When GPUSelector is zero and GPU is
	// non-empty, the string is parsed with ParseGPUSelector (FileBrowser YAML compat).
	GPU string

	// SkipHWTests skips expensive hardware encoder smoke tests.
	SkipHWTests bool

	// VerboseFFmpeg streams ffmpeg stderr to os.Stderr and uses -loglevel info.
	VerboseFFmpeg bool

	// MinVersion rejects ffmpeg binaries older than this during New and Reload.
	// Defaults to capabilities.MinSupportedVersion (5.0.0). Set explicitly when
	// your deployment standard is newer than the library floor.
	MinVersion capabilities.Version

	// ProtocolWhitelist is passed to ffmpeg/ffprobe as -protocol_whitelist before
	// caller-supplied media URLs. When empty, defaults to exec.DefaultProtocolWhitelist
	// ("file,pipe"). Set to exec.DisableProtocolWhitelist ("-") to disable. Extend
	// with network protocols for HTTP/HLS/RTSP inputs, e.g.
	// "file,pipe,http,https,tcp,tls,crypto,rtsp".
	ProtocolWhitelist string
}

// Concurrency configures per-class subprocess limits for Service operations.
type Concurrency = concurrency.Config

// SlotClass identifies which concurrency tier an operation uses.
type SlotClass = concurrency.SlotClass

const (
	// SlotProbe limits ffprobe-style metadata work.
	SlotProbe = concurrency.SlotProbe
	// SlotDecode limits short ffmpeg jobs (previews, screenshots, remux).
	SlotDecode = concurrency.SlotDecode
	// SlotEncode limits heavy transcode / segment encode jobs.
	SlotEncode = concurrency.SlotEncode
)

func (c *Config) withDefaults() Config {
	out := *c
	if out.DetectOnInit == nil {
		t := true
		out.DetectOnInit = &t
	}
	if out.DetectTimeout == 0 {
		out.DetectTimeout = 60 * time.Second
	}
	out.Concurrency = out.resolveConcurrency()
	if out.Logger == nil {
		out.Logger = defaultLogger()
	}
	if out.MinVersion == (capabilities.Version{}) {
		out.MinVersion = capabilities.MinSupportedVersion
	}
	return out
}

func (c *Config) resolveConcurrency() Concurrency {
	cc := c.Concurrency
	hasTier := cc.MaxProbe > 0 || cc.MaxDecode > 0 || cc.MaxEncode > 0 || cc.GlobalMax > 0 ||
		cc.MaxLargeFile != nil || cc.LargeFileThresholdBytes > 0
	if !hasTier {
		if c.MaxConcurrent > 0 {
			return concurrency.LegacyFromMaxConcurrent(c.MaxConcurrent)
		}
		return Concurrency{}.WithDefaults()
	}
	return cc.WithDefaults()
}

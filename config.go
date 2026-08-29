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
	EncoderHierarchy []capabilities.AccelType

	// GPU selects which device to use for hardware encoder smoke tests and
	// HW-aware encode/decode resolution. Empty means software-only: detection
	// still lists compiled HW encoders, but the resolver will not prefer them.
	// Common values: "dgpu", "igpu", a DRM render node (/dev/dri/renderD128),
	// or a substring of the GPU name from the capability report.
	GPU string

	// SkipHWTests skips expensive hardware encoder smoke tests.
	SkipHWTests bool

	// VerboseFFmpeg streams ffmpeg stderr to os.Stderr and uses -loglevel info.
	VerboseFFmpeg bool

	// MinVersion rejects ffmpeg binaries older than this during New and Reload.
	// Defaults to capabilities.MinSupportedVersion (5.0.0). Set explicitly when
	// your deployment standard is newer than the library floor.
	MinVersion capabilities.Version
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

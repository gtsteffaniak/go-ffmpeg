package ffmpeg

import (
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
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

	// MaxConcurrent limits parallel ffmpeg processes. Default 4.
	MaxConcurrent int

	// Logger receives diagnostic output. Inject any implementation of Logger
	// (for example github.com/gtsteffaniak/go-logger/logger.Logger or slog via
	// FromSlog). Defaults to slog.Default() when nil.
	Logger Logger

	// EncoderHierarchy overrides hardware acceleration preference order.
	// When nil or empty, detection uses HierarchyForPlatform (VideoToolbox on macOS, etc.).
	EncoderHierarchy []capabilities.AccelType

	// SkipHWTests skips expensive hardware encoder smoke tests.
	SkipHWTests bool

	// VerboseFFmpeg streams ffmpeg stderr to os.Stderr and uses -loglevel info.
	VerboseFFmpeg bool

	// MinVersion is the minimum required ffmpeg version. Defaults to 5.0.0.
	MinVersion capabilities.Version
}

func (c *Config) withDefaults() Config {
	out := *c
	if out.DetectOnInit == nil {
		t := true
		out.DetectOnInit = &t
	}
	if out.DetectTimeout == 0 {
		out.DetectTimeout = 60 * time.Second
	}
	if out.MaxConcurrent == 0 {
		out.MaxConcurrent = 4
	}
	if out.Logger == nil {
		out.Logger = defaultLogger()
	}
	if out.MinVersion == (capabilities.Version{}) {
		out.MinVersion = capabilities.MinSupportedVersion
	}
	return out
}

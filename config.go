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

	// MaxConcurrent limits how many ffmpeg/ffprobe subprocesses may run at once
	// across all Service methods. Each operation acquires a slot for the duration
	// of the subprocess; StartHLSContinuous holds one slot until Wait completes.
	// Default 4. Use Acquire/Release only for ffmpeg you start outside Service
	// methods (do not call Acquire before Service methods — they manage slots).
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

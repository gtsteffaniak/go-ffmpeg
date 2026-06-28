package encode

import (
	"fmt"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

// ThrottleConfig configures input read pacing for segment encodes.
type ThrottleConfig struct {
	Enabled        bool
	Rate           float64 // -readrate (default 1.0 when enabled)
	Catchup        float64 // -readrate_catchup (default 2.0 when enabled, requires ffmpeg >= 8.0)
	InitialBurst   float64 // -readrate_initial_burst (optional, requires ffmpeg >= 6.1)
	MinDurationSec float64 // reserved for callers; not passed to ffmpeg directly
}

// AppendReadrateArgs appends -readrate flags when cfg.Enabled and the version supports them.
func AppendReadrateArgs(args []string, ver capabilities.Version, cfg ThrottleConfig) []string {
	if !cfg.Enabled {
		return args
	}
	flags := capabilities.FeatureFlagsFromVersion(ver)
	rate := cfg.Rate
	if rate <= 0 {
		rate = 1.0
	}
	if flags.Readrate {
		args = append(args, "-readrate", fmt.Sprintf("%g", rate))
	}
	catchup := cfg.Catchup
	if catchup <= 0 {
		catchup = 2.0
	}
	if flags.ReadrateCatchup {
		args = append(args, "-readrate_catchup", fmt.Sprintf("%g", catchup))
	}
	if cfg.InitialBurst > 0 && capabilities.Compare(ver, capabilities.Version{Major: 6, Minor: 1, Patch: 0}) >= 0 {
		args = append(args, "-readrate_initial_burst", fmt.Sprintf("%g", cfg.InitialBurst))
	}
	return args
}

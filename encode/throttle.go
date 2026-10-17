package encode

import (
	"fmt"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

// ThrottleConfig configures input read pacing for segment encodes.
type ThrottleConfig struct {
	Enabled        bool
	Rate           float64 // -readrate (default 1.0 when enabled)
	Catchup        float64 // -readrate_catchup when > 0 (requires ffmpeg >= 8.0); 0 omits flag (ffmpeg 8+ default readrate×1.05)
	InitialBurst   float64 // -readrate_initial_burst (optional, requires ffmpeg >= 6.1)
	MinDurationSec float64 // reserved for callers; not passed to ffmpeg directly
}

// AppendReadrateArgs appends -readrate flags when cfg.Enabled.
// Base -readrate is always emitted for supported ffmpeg (min 5.0). Catchup is appended only
// when cfg.Catchup > 0 (8.0+); otherwise ffmpeg applies its default catchup rate. initial_burst
// requires 6.1+.
func AppendReadrateArgs(args []string, ver capabilities.Version, cfg ThrottleConfig) []string {
	if !cfg.Enabled {
		return args
	}
	rate := cfg.Rate
	if rate <= 0 {
		rate = 1.0
	}
	args = append(args, "-readrate", fmt.Sprintf("%g", rate))
	flags := capabilities.FeatureFlagsFromVersion(ver)
	if cfg.Catchup > 0 && flags.ReadrateCatchup {
		args = append(args, "-readrate_catchup", fmt.Sprintf("%g", cfg.Catchup))
	}
	if cfg.InitialBurst > 0 && capabilities.Compare(ver, capabilities.Version{Major: 6, Minor: 1, Patch: 0}) >= 0 {
		args = append(args, "-readrate_initial_burst", fmt.Sprintf("%g", cfg.InitialBurst))
	}
	return args
}

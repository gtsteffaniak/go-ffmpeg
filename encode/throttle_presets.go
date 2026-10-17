package encode

// DefaultLivePacedBurstSec is the default readrate_initial_burst for live-paced jobs
// (~5 segments at 4s). Pair with app-level max-ahead throttling for long sessions.
const DefaultLivePacedBurstSec = 20

// ThrottleConfigOff returns a disabled throttle (max encode speed — use for background disk cache).
func ThrottleConfigOff() ThrottleConfig {
	return ThrottleConfig{Enabled: false}
}

// ThrottleConfigLivePaced returns ~1x input pacing after an initial unlimited burst.
// Catchup is left at 0 so ffmpeg 8+ applies its default readrate×1.05 recovery; set Catchup
// explicitly on ThrottleConfig when a higher cap is needed. Pair with app-level pause when
// segments are far ahead of playback.
func ThrottleConfigLivePaced(initialBurstSec float64) ThrottleConfig {
	if initialBurstSec <= 0 {
		initialBurstSec = DefaultLivePacedBurstSec
	}
	return ThrottleConfig{
		Enabled:      true,
		Rate:         1.0,
		Catchup:      0,
		InitialBurst: initialBurstSec,
	}
}

// ThrottleConfigRemuxSegmentDeletion matches Jellyfin stream-copy + segment deletion pacing
// (-readrate 10 with high catchup so remux does not EOF before the throttler runs).
func ThrottleConfigRemuxSegmentDeletion() ThrottleConfig {
	return ThrottleConfig{
		Enabled: true,
		Rate:    10,
		Catchup: 1000,
	}
}

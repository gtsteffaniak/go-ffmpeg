package encode

// DefaultLivePacedBurstSec is the default readrate_initial_burst for live-paced jobs
// (~15 segments at 4s). Align with app-level buffer settings (Jellyfin default throttle is 180s).
const DefaultLivePacedBurstSec = 60

// ThrottleConfigOff returns a disabled throttle (max encode speed — use for background disk cache).
func ThrottleConfigOff() ThrottleConfig {
	return ThrottleConfig{Enabled: false}
}

// ThrottleConfigLivePaced returns slightly-faster-than-realtime input pacing
// after an initial unlimited burst. ~1.5x keeps the produced edge ahead of
// playback (1x reads exactly at consumption speed, so any stall starves the
// playhead) while the cache still only fills for content that is watched.
func ThrottleConfigLivePaced(initialBurstSec float64) ThrottleConfig {
	if initialBurstSec <= 0 {
		initialBurstSec = DefaultLivePacedBurstSec
	}
	return ThrottleConfig{
		Enabled:      true,
		Rate:         1.5,
		Catchup:      2.0,
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

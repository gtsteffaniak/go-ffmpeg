package ops

import "github.com/gtsteffaniak/go-ffmpeg/encode"

// HLSContinuousPacing selects input read pacing for a continuous HLS job.
// A non-nil Throttle on HLSContinuousOptions overrides pacing presets,
// including when Throttle.Enabled is false.
type HLSContinuousPacing int

const (
	// HLSContinuousCacheFill encodes as fast as possible. Background disk cache prefetch.
	HLSContinuousCacheFill HLSContinuousPacing = 0
	// HLSContinuousLivePaced paces input at ~1x after an initial burst. Watch-while-transcode.
	HLSContinuousLivePaced HLSContinuousPacing = 1
	// HLSContinuousRemuxSegmentDeletion uses high readrate for stream-copy with segment deletion.
	HLSContinuousRemuxSegmentDeletion HLSContinuousPacing = 2
	// HLSContinuousPacingAuto picks live-paced for full stream-copy remux and cache-fill otherwise.
	HLSContinuousPacingAuto HLSContinuousPacing = 3
)

// ResolveContinuousPacing returns the effective pacing mode for opts.
func ResolveContinuousPacing(opts HLSContinuousOptions) HLSContinuousPacing {
	switch opts.Pacing {
	case HLSContinuousCacheFill, HLSContinuousLivePaced, HLSContinuousRemuxSegmentDeletion:
		return opts.Pacing
	case HLSContinuousPacingAuto:
		return resolveAutoContinuousPacing(opts)
	default:
		return HLSContinuousCacheFill
	}
}

func resolveAutoContinuousPacing(opts HLSContinuousOptions) HLSContinuousPacing {
	// Full remux (video+audio copy) can sustain live-paced ~1x readrate.
	if opts.Remux {
		return HLSContinuousLivePaced
	}
	// Video copy with audio transcode, full transcode, or scaled encode need max throughput.
	return HLSContinuousCacheFill
}

func resolveContinuousThrottle(opts HLSContinuousOptions) encode.ThrottleConfig {
	if opts.Throttle != nil {
		return *opts.Throttle
	}
	switch ResolveContinuousPacing(opts) {
	case HLSContinuousLivePaced:
		return encode.ThrottleConfigLivePaced(encode.DefaultLivePacedBurstSec)
	case HLSContinuousRemuxSegmentDeletion:
		return encode.ThrottleConfigRemuxSegmentDeletion()
	default:
		return encode.ThrottleConfigOff()
	}
}

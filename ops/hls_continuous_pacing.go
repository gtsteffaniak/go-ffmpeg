package ops

import "github.com/gtsteffaniak/go-ffmpeg/encode"

// HLSContinuousPacing selects input read pacing for a continuous HLS job.
// Explicit Throttle.Enabled on HLSContinuousOptions overrides pacing presets.
type HLSContinuousPacing int

const (
	// HLSContinuousCacheFill encodes as fast as possible (default). Background disk cache prefetch.
	HLSContinuousCacheFill HLSContinuousPacing = 0
	// HLSContinuousLivePaced paces input at ~1x after an initial burst. Watch-while-transcode.
	HLSContinuousLivePaced HLSContinuousPacing = 1
	// HLSContinuousRemuxSegmentDeletion uses high readrate for stream-copy with segment deletion.
	HLSContinuousRemuxSegmentDeletion HLSContinuousPacing = 2
)

func resolveContinuousThrottle(opts HLSContinuousOptions) encode.ThrottleConfig {
	if opts.Throttle.Enabled {
		return opts.Throttle
	}
	switch opts.Pacing {
	case HLSContinuousLivePaced:
		return encode.ThrottleConfigLivePaced(encode.DefaultLivePacedBurstSec)
	case HLSContinuousRemuxSegmentDeletion:
		return encode.ThrottleConfigRemuxSegmentDeletion()
	default:
		return encode.ThrottleConfigOff()
	}
}

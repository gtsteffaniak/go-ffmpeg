package ops

// DefaultHLSSegmentDurationSec is the on-demand fMP4 segment target length.
const DefaultHLSSegmentDurationSec = 4.0

const defaultHLSDefaultGOP = 120

// OnDemandHLSDefaults holds segment timing defaults for independent on-demand encodes.
type OnDemandHLSDefaults struct {
	SegmentDurationSec float64
	DefaultGOP         int
}

// DefaultOnDemandHLSDefaults returns validated on-demand segment defaults.
func DefaultOnDemandHLSDefaults() OnDemandHLSDefaults {
	return OnDemandHLSDefaults{
		SegmentDurationSec: DefaultHLSSegmentDurationSec,
		DefaultGOP:         defaultHLSDefaultGOP,
	}
}

// Normalized returns d with default fields filled in.
func (d OnDemandHLSDefaults) Normalized() OnDemandHLSDefaults {
	out := d
	if out.SegmentDurationSec <= 0 {
		out.SegmentDurationSec = DefaultHLSSegmentDurationSec
	}
	if out.DefaultGOP <= 0 {
		out.DefaultGOP = defaultHLSDefaultGOP
	}
	return out
}

// HLSSegmentGOP returns GOP size from fps and segment duration, or DefaultGOP when fps is unknown.
func HLSSegmentGOP(fps float64, defaults OnDemandHLSDefaults) int {
	defaults = defaults.Normalized()
	if fps <= 0 {
		return defaults.DefaultGOP
	}
	gop := int(fps * defaults.SegmentDurationSec)
	if gop < 1 {
		return defaults.DefaultGOP
	}
	return gop
}

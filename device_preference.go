package ffmpeg

import (
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

// DevicePreference controls default GPU device order when DeviceSelection is unset.
type DevicePreference int

const (
	// PreferIntegrated (zero) tries integrated/SoC GPUs first, then discrete, then software.
	PreferIntegrated DevicePreference = 0
	// PreferDiscrete tries discrete GPUs first, then integrated/SoC, then software.
	PreferDiscrete DevicePreference = 1
)

// String returns a stable label for reports.
func (p DevicePreference) String() string {
	switch p {
	case PreferDiscrete:
		return "discrete"
	default:
		return "integrated"
	}
}

func (c *Config) effectiveDevicePreference() DevicePreference {
	if c.DevicePreference == PreferDiscrete {
		return PreferDiscrete
	}
	if pref, err := capabilities.ParseEncoderPreference(c.EncoderPreference); err == nil {
		if pref == capabilities.EncoderPreferencePerformance {
			return PreferDiscrete
		}
	}
	return PreferIntegrated
}

package encode

import "github.com/gtsteffaniak/go-ffmpeg/capabilities"

// ApplyEncodeDefaults fills empty Accel / ForceSoftware from service-level PreferredAccel.
func ApplyEncodeDefaults(profile VideoProfile, preferred capabilities.AccelType) VideoProfile {
	if profile.ForceSoftware || profile.Encoder != "" || profile.Accel != "" {
		return profile
	}
	if preferred == capabilities.AccelNone {
		profile.ForceSoftware = true
		return profile
	}
	if preferred != "" {
		profile.Accel = preferred
	}
	return profile
}

// ApplyDecodeDefaults fills empty Accel from service-level PreferredAccel.
// ForceSoftware profiles are left unchanged.
func ApplyDecodeDefaults(profile VideoDecodeProfile, preferred capabilities.AccelType) VideoDecodeProfile {
	if profile.ForceSoftware || profile.Accel != "" {
		return profile
	}
	if preferred == capabilities.AccelNone {
		profile.ForceSoftware = true
		return profile
	}
	if preferred != "" {
		profile.Accel = preferred
	}
	return profile
}

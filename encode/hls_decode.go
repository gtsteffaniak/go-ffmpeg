package encode

import "github.com/gtsteffaniak/go-ffmpeg/capabilities"

// HLSTranscodeDecodeProfile normalizes input decode for HLS transcode pipelines.
// Hardware decode is preferred when available; callers may set ForceSoftware explicitly.
func HLSTranscodeDecodeProfile(profile VideoDecodeProfile) VideoDecodeProfile {
	return profile
}

// OnDemandHLSDecodeProfile is an alias for HLSTranscodeDecodeProfile.
func OnDemandHLSDecodeProfile(profile VideoDecodeProfile) VideoDecodeProfile {
	return HLSTranscodeDecodeProfile(profile)
}

// SoftwareDecodeProfile returns a CPU decode profile for the same codec.
func SoftwareDecodeProfile(profile VideoDecodeProfile) VideoDecodeProfile {
	return VideoDecodeProfile{
		Codec: profile.Codec,
		Accel: capabilities.AccelNone,
	}
}

package encode

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// HLSDecodeProfileForOnDemand selects input decode for short on-demand HLS segments.
func HLSDecodeProfileForOnDemand(info probe.StreamInfo) VideoDecodeProfile {
	if !isKnownHLSInputVideoCodec(info.VideoCodec) {
		return VideoDecodeProfile{ForceSoftware: true}
	}
	return OnDemandHLSDecodeProfile(VideoDecodeProfile{
		Codec: probeVideoCodec(info.VideoCodec),
	})
}

// DefaultHLSVideoProfile returns safe H.264 transcode defaults when the caller
// does not supply encode settings (medium preset, resolution-aware bitrate).
func DefaultHLSVideoProfile(maxHeight int) VideoProfile {
	targetKbps := defaultHLSBitrateKbps(maxHeight)
	return VideoProfile{
		Codec:   CodecH264,
		Quality: PresetMedium,
		Bitrate: BitrateConfig{
			Target:  fmt.Sprintf("%dk", targetKbps),
			Min:     fmt.Sprintf("%dk", targetKbps/2),
			Max:     fmt.Sprintf("%dk", targetKbps*3/2),
			BufSize: fmt.Sprintf("%dk", targetKbps*2),
		},
	}
}

func defaultHLSBitrateKbps(maxHeight int) int {
	switch {
	case maxHeight >= 1080:
		return 5000
	case maxHeight >= 720:
		return 3500
	case maxHeight >= 480:
		return 2000
	default:
		return 1500
	}
}

func isKnownHLSInputVideoCodec(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "h264", "avc", "avc1", "hevc", "h265", "vp9", "av1":
		return true
	default:
		return false
	}
}

func probeVideoCodec(name string) capabilities.VideoCodec {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "h264", "avc", "avc1":
		return capabilities.CodecH264
	case "hevc", "h265":
		return capabilities.CodecHEVC
	case "vp9":
		return capabilities.CodecVP9
	case "av1":
		return capabilities.CodecAV1
	default:
		return capabilities.CodecH264
	}
}

func VideoProfileIsEmpty(p VideoProfile) bool {
	return p.Codec == ""
}

func VideoDecodeProfileIsEmpty(p VideoDecodeProfile) bool {
	return p.Codec == "" && !p.ForceSoftware && p.Accel == "" && p.Decoder == ""
}

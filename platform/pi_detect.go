package platform

import (
	"os"
	"strings"
)

// PiModel identifies Raspberry Pi hardware generation when running on Linux ARM.
type PiModel string

const (
	PiUnknown PiModel = ""
	// PiLegacy is Raspberry Pi 3 and earlier (no usable HW encode path; OMX/MMAL removed).
	PiLegacy PiModel = "pi_legacy"
	Pi4      PiModel = "pi4"
	Pi5      PiModel = "pi5"
)

// DetectPiModel reads device-tree model strings on Linux ARM boards.
func DetectPiModel() PiModel {
	if os.Getenv("GOFFMPEG_PI_MODEL") != "" {
		switch strings.ToLower(strings.TrimSpace(os.Getenv("GOFFMPEG_PI_MODEL"))) {
		case "pi4", "4":
			return Pi4
		case "pi5", "5":
			return Pi5
		}
	}
	model := readDeviceTreeModel()
	lower := strings.ToLower(model)
	if strings.Contains(lower, "raspberry pi 5") {
		return Pi5
	}
	if strings.Contains(lower, "raspberry pi 4") {
		return Pi4
	}
	if strings.Contains(lower, "raspberry pi") {
		return PiLegacy
	}
	return PiUnknown
}

func readDeviceTreeModel() string {
	for _, path := range []string{
		"/proc/device-tree/model",
		"/sys/firmware/devicetree/base/model",
	} {
		if data, err := os.ReadFile(path); err == nil {
			return strings.Trim(strings.TrimSpace(string(data)), "\x00")
		}
	}
	return ""
}

// PiCapabilities summarizes hardware codec support for the detected Pi model.
func PiCapabilities(model PiModel) (h264Encode, h264Decode, hevcDecode bool, maxH264EncodeHeight int) {
	switch model {
	case Pi4:
		return true, true, true, 1080
	case Pi5:
		return false, false, true, 0
	case PiLegacy:
		return false, false, false, 0
	default:
		return false, false, false, 0
	}
}

// PiEncodeProfile holds reported defaults for HLS-oriented H.264 on Raspberry Pi OS.
// Callers may override; these are recommendations from capability detection only.
type PiEncodeProfile struct {
	Model                PiModel `json:"model"`
	H264Summary          string  `json:"h264Summary"`
	DefaultH264Encoder   string  `json:"defaultH264Encoder"`
	DefaultH264Preset    string  `json:"defaultH264Preset,omitempty"`
	RecommendedMaxHeight int     `json:"recommendedMaxHeight,omitempty"`
	RecommendedMaxFPS    int     `json:"recommendedMaxFPS,omitempty"`
	MaxHWEncodeHeight    int     `json:"maxHWEncodeHeight,omitempty"`
	MaxEncodeConcurrency int     `json:"maxEncodeConcurrency,omitempty"`
	HEVCDecodeHW         bool    `json:"hevcDecodeHW,omitempty"`
	UseBitrateForV4L2M2M bool    `json:"useBitrateForV4L2M2M,omitempty"`
	InstallHint          string  `json:"installHint,omitempty"`
}

// PiEncodeProfileForModel returns capability-reported encode guidance for Pi boards.
func PiEncodeProfileForModel(model PiModel) PiEncodeProfile {
	switch model {
	case Pi5:
		return PiEncodeProfile{
			Model:                Pi5,
			H264Summary:          "H.264: software (no hardware encoder on Raspberry Pi 5)",
			DefaultH264Encoder:   "libx264",
			DefaultH264Preset:    "veryfast",
			RecommendedMaxHeight: 720,
			RecommendedMaxFPS:    30,
			MaxEncodeConcurrency: 1,
			HEVCDecodeHW:         true,
			InstallHint:          "Use Raspberry Pi OS ffmpeg (apt full upgrade); HEVC decode may use hevc_v4l2request when available",
		}
	case Pi4:
		return PiEncodeProfile{
			Model:                Pi4,
			H264Summary:          "H.264: h264_v4l2m2m (bitrate-based, up to ~1080p30) with libx264 fallback above limits",
			DefaultH264Encoder:   "h264_v4l2m2m",
			RecommendedMaxHeight: 1080,
			RecommendedMaxFPS:    30,
			MaxHWEncodeHeight:    1080,
			MaxEncodeConcurrency: 2,
			HEVCDecodeHW:         true,
			UseBitrateForV4L2M2M: true,
			InstallHint:          "Use Raspberry Pi OS patched ffmpeg (not stock Debian arm64 builds without v4l2m2m)",
		}
	case PiLegacy:
		return PiEncodeProfile{
			Model:                PiLegacy,
			H264Summary:          "H.264: software only (OMX/MMAL removed; no v4l2m2m encode on Pi 3 and older)",
			DefaultH264Encoder:   "libx264",
			DefaultH264Preset:    "ultrafast",
			RecommendedMaxHeight: 720,
			RecommendedMaxFPS:    25,
			MaxEncodeConcurrency: 1,
			InstallHint:          "Use libx264; hardware OMX/MMAL encoders are not available in modern FFmpeg",
		}
	default:
		return PiEncodeProfile{}
	}
}

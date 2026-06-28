package capabilities

import "strings"

// KnownDecoders lists decoders probed during capability detection.
var KnownDecoders = []struct {
	Name    string
	Kind    string
	Codec   VideoCodec
	HW      bool
	HWAccel string // non-empty for hwaccel-based decode paths (e.g. vaapi)
}{
	// Software codec decoders
	{"h264", "software", CodecH264, false, ""},
	{"hevc", "software", CodecHEVC, false, ""},
	{"vp9", "software", CodecVP9, false, ""},
	{"av1", "software", CodecAV1, false, ""},
	// NVIDIA CUVID (NVDEC)
	{"h264_cuvid", "nvenc", CodecH264, true, ""},
	{"hevc_cuvid", "nvenc", CodecHEVC, true, ""},
	{"av1_cuvid", "nvenc", CodecAV1, true, ""},
	{"vp9_cuvid", "nvenc", CodecVP9, true, ""},
	// Intel QSV decode (same ffmpeg decoder names as QSV encode)
	{"h264_qsv", "qsv", CodecH264, true, ""},
	{"hevc_qsv", "qsv", CodecHEVC, true, ""},
	{"av1_qsv", "qsv", CodecAV1, true, ""},
	{"vp9_qsv", "qsv", CodecVP9, true, ""},
	// VAAPI hwaccel decode (no dedicated *_vaapi decoder in FFmpeg)
	{"hwaccel:vaapi:h264", "vaapi", CodecH264, true, "vaapi"},
	{"hwaccel:vaapi:hevc", "vaapi", CodecHEVC, true, "vaapi"},
	{"hwaccel:vaapi:vp9", "vaapi", CodecVP9, true, "vaapi"},
	{"hwaccel:vaapi:av1", "vaapi", CodecAV1, true, "vaapi"},
	// VideoToolbox hwaccel decode (macOS)
	{"hwaccel:videotoolbox:h264", "videotoolbox", CodecH264, true, "videotoolbox"},
	{"hwaccel:videotoolbox:hevc", "videotoolbox", CodecHEVC, true, "videotoolbox"},
}

// DecodeBinding links an encoder row to its decode capability entry.
type DecodeBinding struct {
	Key     string
	Label   string
	HWAccel string
	SWCodec string
}

// DecodeBindingForEncoder returns the decoder capability key for a report encoder row.
func DecodeBindingForEncoder(encoderName string) (DecodeBinding, bool) {
	switch encoderName {
	case "libx264":
		return DecodeBinding{Key: "h264", Label: "h264"}, true
	case "libx265", "libvvenc":
		return DecodeBinding{Key: "hevc", Label: "hevc"}, true
	case "libvpx-vp9":
		return DecodeBinding{Key: "vp9", Label: "vp9"}, true
	case "libsvtav1", "librav1e", "libaom-av1":
		return DecodeBinding{Key: "av1", Label: "av1"}, true
	case "h264_nvenc":
		return DecodeBinding{Key: "h264_cuvid", Label: "h264_cuvid"}, true
	case "hevc_nvenc":
		return DecodeBinding{Key: "hevc_cuvid", Label: "hevc_cuvid"}, true
	case "av1_nvenc":
		return DecodeBinding{Key: "av1_cuvid", Label: "av1_cuvid"}, true
	case "vp9_nvenc":
		return DecodeBinding{Key: "vp9_cuvid", Label: "vp9_cuvid"}, true
	case "h264_qsv":
		return DecodeBinding{Key: "h264_qsv", Label: "h264_qsv"}, true
	case "hevc_qsv":
		return DecodeBinding{Key: "hevc_qsv", Label: "hevc_qsv"}, true
	case "av1_qsv":
		return DecodeBinding{Key: "av1_qsv", Label: "av1_qsv"}, true
	case "vp9_qsv":
		return DecodeBinding{Key: "vp9_qsv", Label: "vp9_qsv"}, true
	case "h264_vaapi":
		return DecodeBinding{Key: "hwaccel:vaapi:h264", Label: "vaapi+h264", HWAccel: "vaapi", SWCodec: "h264"}, true
	case "hevc_vaapi":
		return DecodeBinding{Key: "hwaccel:vaapi:hevc", Label: "vaapi+hevc", HWAccel: "vaapi", SWCodec: "hevc"}, true
	case "av1_vaapi":
		return DecodeBinding{Key: "hwaccel:vaapi:av1", Label: "vaapi+av1", HWAccel: "vaapi", SWCodec: "av1"}, true
	case "vp9_vaapi":
		return DecodeBinding{Key: "hwaccel:vaapi:vp9", Label: "vaapi+vp9", HWAccel: "vaapi", SWCodec: "vp9"}, true
	case "h264_videotoolbox":
		return DecodeBinding{Key: "hwaccel:videotoolbox:h264", Label: "videotoolbox+h264", HWAccel: "videotoolbox", SWCodec: "h264"}, true
	case "hevc_videotoolbox":
		return DecodeBinding{Key: "hwaccel:videotoolbox:hevc", Label: "videotoolbox+hevc", HWAccel: "videotoolbox", SWCodec: "hevc"}, true
	default:
		return DecodeBinding{}, false
	}
}

// CodecDecoderMap maps logical codec + accel to ffmpeg decoder name (empty for hwaccel paths).
func CodecDecoderMap(codec VideoCodec, accel AccelType) string {
	switch codec {
	case CodecH264:
		switch accel {
		case AccelNVENC:
			return "h264_cuvid"
		case AccelQSV:
			return "h264_qsv"
		case AccelVAAPI, AccelD3D12:
			return ""
		}
	case CodecHEVC:
		switch accel {
		case AccelNVENC:
			return "hevc_cuvid"
		case AccelQSV:
			return "hevc_qsv"
		case AccelVAAPI, AccelD3D12:
			return ""
		}
	case CodecAV1:
		switch accel {
		case AccelNVENC:
			return "av1_cuvid"
		case AccelQSV:
			return "av1_qsv"
		case AccelVAAPI, AccelD3D12:
			return ""
		}
	case CodecVP9:
		switch accel {
		case AccelNVENC:
			return "vp9_cuvid"
		case AccelQSV:
			return "vp9_qsv"
		case AccelVAAPI, AccelD3D12:
			return ""
		}
	}
	return ""
}

// CodecHWAccelDecodeKey returns the Decoders map key for hwaccel-based decode.
func CodecHWAccelDecodeKey(codec VideoCodec, accel AccelType) string {
	sw := softwareCodecName(codec)
	if sw == "" {
		return ""
	}
	switch accel {
	case AccelVAAPI:
		return "hwaccel:vaapi:" + sw
	case AccelD3D12:
		return "hwaccel:vaapi:" + sw
	case AccelVideoToolbox:
		return "hwaccel:videotoolbox:" + sw
	default:
		return ""
	}
}

// SoftwareDecodeFallback returns software decoder names for a codec.
func SoftwareDecodeFallback(codec VideoCodec) []string {
	if name := softwareCodecName(codec); name != "" {
		return []string{name}
	}
	return nil
}

func softwareCodecName(codec VideoCodec) string {
	switch codec {
	case CodecH264:
		return "h264"
	case CodecHEVC:
		return "hevc"
	case CodecVP9:
		return "vp9"
	case CodecAV1:
		return "av1"
	default:
		return ""
	}
}

// IsHardwareDecoder reports whether a decoder entry requires runtime smoke testing.
func IsHardwareDecoder(name string) bool {
	for _, d := range KnownDecoders {
		if d.Name == name {
			return d.HW
		}
	}
	return strings.HasPrefix(name, "hwaccel:")
}

func decoderKindForName(name string) string {
	return DecoderKindForName(name)
}

// DecoderKindForName returns the backend kind for a decoder capability key.
func DecoderKindForName(name string) string {
	for _, d := range KnownDecoders {
		if d.Name == name {
			return d.Kind
		}
	}
	if strings.HasPrefix(name, "hwaccel:vaapi:") {
		return "vaapi"
	}
	if strings.HasPrefix(name, "hwaccel:videotoolbox:") {
		return "videotoolbox"
	}
	if strings.Contains(name, "cuvid") {
		return "nvenc"
	}
	if strings.Contains(name, "_qsv") {
		return "qsv"
	}
	return "unknown"
}

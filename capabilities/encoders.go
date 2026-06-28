package capabilities

// KnownEncoders lists encoders the library probes during detection.
var KnownEncoders = []struct {
	Name string
	Kind string
	HW   bool
}{
	// Software H.264/HEVC/AV1/VP9
	{"libx264", "software", false},
	{"libx265", "software", false},
	{"libsvtav1", "software", false},
	{"librav1e", "software", false},
	{"libaom-av1", "software", false},
	{"libvpx-vp9", "software", false},
	{"libvvenc", "software", false},
	// Native
	{"mjpeg", "native", false},
	{"aac", "native", false},
	{"libmp3lame", "software", false},
	// NVENC
	{"h264_nvenc", "nvenc", true},
	{"hevc_nvenc", "nvenc", true},
	{"av1_nvenc", "nvenc", true},
	{"vp9_nvenc", "nvenc", true},
	// AMF
	{"h264_amf", "amf", true},
	{"av1_amf", "amf", true},
	{"vp9_amf", "amf", true},
	// QSV
	{"h264_qsv", "qsv", true},
	{"hevc_qsv", "qsv", true},
	{"av1_qsv", "qsv", true},
	{"vp9_qsv", "qsv", true},
	// VAAPI (d3d12 path in WSL)
	{"h264_vaapi", "vaapi", true},
	{"av1_vaapi", "vaapi", true},
	{"vp9_vaapi", "vaapi", true},
	{"hevc_vaapi", "vaapi", true},
	// VideoToolbox (macOS)
	{"h264_videotoolbox", "videotoolbox", true},
	{"hevc_videotoolbox", "videotoolbox", true},
}

// KnownFilters lists filters required by library operations.
var KnownFilters = []string{
	"scale", "format", "segment", "concat", "tile",
	"transpose", "hflip", "vflip", "fps", "setpts",
}

// KnownProtocols lists protocols required by library operations.
var KnownProtocols = []string{
	"file", "http", "https", "rtsp", "hls", "tcp", "pipe",
}

// CodecEncoderMap maps logical codec + accel to ffmpeg encoder name.
func CodecEncoderMap(codec VideoCodec, accel AccelType) string {
	switch codec {
	case CodecH264:
		switch accel {
		case AccelNVENC:
			return "h264_nvenc"
		case AccelAMF:
			return "h264_amf"
		case AccelQSV:
			return "h264_qsv"
		case AccelVAAPI, AccelD3D12:
			return "h264_vaapi"
		case AccelVideoToolbox:
			return "h264_videotoolbox"
		}
	case CodecAV1:
		switch accel {
		case AccelNVENC:
			return "av1_nvenc"
		case AccelAMF:
			return "av1_amf"
		case AccelQSV:
			return "av1_qsv"
		case AccelVAAPI, AccelD3D12:
			return "av1_vaapi"
		}
	case CodecVP9:
		switch accel {
		case AccelNVENC:
			return "vp9_nvenc"
		case AccelAMF:
			return "vp9_amf"
		case AccelQSV:
			return "vp9_qsv"
		case AccelVAAPI, AccelD3D12:
			return "vp9_vaapi"
		}
	case CodecHEVC:
		switch accel {
		case AccelNVENC:
			return "hevc_nvenc"
		case AccelQSV:
			return "hevc_qsv"
		case AccelVAAPI, AccelD3D12:
			return "hevc_vaapi"
		case AccelVideoToolbox:
			return "hevc_videotoolbox"
		}
	}
	return ""
}

// SoftwareFallback returns software encoder names for a codec, in preference order.
func SoftwareFallback(codec VideoCodec) []string {
	switch codec {
	case CodecH264:
		return []string{"libx264"}
	case CodecAV1:
		return []string{"libsvtav1", "librav1e", "libaom-av1"}
	case CodecVP9:
		return []string{"libvpx-vp9"}
	case CodecHEVC:
		return []string{"libx265", "libvvenc"}
	default:
		return nil
	}
}

// IsHardwareEncoder reports whether an encoder requires smoke testing.
func IsHardwareEncoder(name string) bool {
	for _, e := range KnownEncoders {
		if e.Name == name {
			return e.HW
		}
	}
	return false
}

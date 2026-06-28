package capabilities

import "strings"

// DecoderLabel returns a human-readable decoder description.
func DecoderLabel(name string) string {
	if label, ok := decoderLabels[name]; ok {
		return label
	}
	if strings.HasPrefix(name, "hwaccel:vaapi:") {
		return "VAAPI decode (" + strings.TrimPrefix(name, "hwaccel:vaapi:") + ")"
	}
	return name
}

var decoderLabels = map[string]string{
	"h264":       "H.264 (software decode)",
	"hevc":       "HEVC (software decode)",
	"vp9":        "VP9 (software decode)",
	"av1":        "AV1 (software decode)",
	"h264_cuvid": "H.264 — NVDEC",
	"hevc_cuvid": "HEVC — NVDEC",
	"av1_cuvid":  "AV1 — NVDEC",
	"vp9_cuvid":  "VP9 — NVDEC",
	"h264_qsv":   "H.264 — QSV decode",
	"hevc_qsv":   "HEVC — QSV decode",
	"av1_qsv":    "AV1 — QSV decode",
	"vp9_qsv":    "VP9 — QSV decode",
}

// AccelLabel returns a human-readable hardware acceleration name.
func AccelLabel(accel AccelType) string {
	if label, ok := accelLabels[accel]; ok {
		return label
	}
	return string(accel)
}

// CodecLabel returns a human-readable video codec name.
func CodecLabel(codec VideoCodec) string {
	if label, ok := codecLabels[codec]; ok {
		return label
	}
	return string(codec)
}

// EncoderLabel returns a human-readable encoder description.
func EncoderLabel(name string) string {
	if label, ok := encoderLabels[name]; ok {
		return label
	}
	return name
}

// EncoderKindLabel returns a human-readable encoder backend name.
func EncoderKindLabel(kind string) string {
	if label, ok := encoderKindLabels[kind]; ok {
		return label
	}
	return kind
}

// PlatformGateLabel returns a human-readable platform gate name.
func PlatformGateLabel(gate string) string {
	if label, ok := platformGateLabels[gate]; ok {
		return label
	}
	return gate
}

// BackendDisplayLabel returns the backend name for grouped report rows (no codec prefix).
func BackendDisplayLabel(name, kind string) string {
	if label, ok := backendLabels[name]; ok {
		return label
	}
	switch kind {
	case "software":
		return "Software"
	case "native":
		return "Built-in"
	case "nvenc":
		return "NVIDIA NVENC"
	case "amf":
		return "AMD AMF"
	case "qsv":
		return "Intel Quick Sync"
	case "vaapi":
		return "VAAPI"
	case "videotoolbox":
		return "VideoToolbox"
	default:
		return kind
	}
}

var backendLabels = map[string]string{
	"libx264":    "Software — x264",
	"libx265":    "Software — x265",
	"libsvtav1":  "Software — SVT-AV1",
	"librav1e":   "Software — rav1e",
	"libaom-av1": "Software — libaom",
	"libvpx-vp9": "Software — libvpx",
	"libvvenc":   "Software — vvenc",
	"mjpeg":      "Built-in — Motion JPEG",
	"aac":        "Built-in — AAC",
	"libmp3lame": "Software — LAME (MP3)",
}

var accelLabels = map[AccelType]string{
	AccelNVENC:        "NVIDIA NVENC",
	AccelAMF:          "AMD AMF",
	AccelQSV:          "Intel Quick Sync Video",
	AccelVAAPI:        "VAAPI",
	AccelD3D12:        "WSL D3D12 (VAAPI)",
	AccelVideoToolbox: "Apple VideoToolbox",
	AccelNone:         "Software",
}

var codecLabels = map[VideoCodec]string{
	CodecH264: "H.264 / AVC",
	CodecAV1:  "AV1",
	CodecVP9:  "VP9",
	CodecHEVC: "H.265 / HEVC",
}

var encoderKindLabels = map[string]string{
	"software":     "Software encoder",
	"native":       "FFmpeg built-in",
	"nvenc":        "NVIDIA NVENC",
	"amf":          "AMD AMF",
	"qsv":          "Intel Quick Sync Video",
	"vaapi":        "VAAPI (Intel/AMD GPU)",
	"videotoolbox": "Apple VideoToolbox",
}

var platformGateLabels = map[string]string{
	"NVIDIA":     "NVIDIA GPU + driver",
	"AMD":        "AMD GPU",
	"Intel":      "Intel GPU",
	"DRI":        "DRM render nodes",
	"QSV":        "Intel Quick Sync Video",
	"QSVRuntime": "oneVPL GPU runtime (libmfx-gen)",
	"VPL":        "oneVPL dispatcher (libvpl2)",
	"VAAPI":      "VAAPI driver stack",
	"D3D12":      "WSL D3D12 / DXGK",
	"WSL":        "Windows Subsystem for Linux",
}

var encoderLabels = map[string]string{
	// Software
	"libx264":    "H.264 — x264 (software)",
	"libx265":    "HEVC — x265 (software)",
	"libsvtav1":  "AV1 — SVT-AV1 (software)",
	"librav1e":   "AV1 — rav1e (software)",
	"libaom-av1": "AV1 — libaom (software)",
	"libvpx-vp9": "VP9 — libvpx (software)",
	"libvvenc":   "HEVC — vvenc (software)",
	"libmp3lame": "MP3 — LAME (software)",
	"mjpeg":      "Motion JPEG (built-in)",
	"aac":        "AAC (built-in)",
	// NVENC
	"h264_nvenc": "H.264 — NVIDIA NVENC",
	"hevc_nvenc": "HEVC — NVIDIA NVENC",
	"av1_nvenc":  "AV1 — NVIDIA NVENC",
	"vp9_nvenc":  "VP9 — NVIDIA NVENC",
	// AMF
	"h264_amf": "H.264 — AMD AMF",
	"av1_amf":  "AV1 — AMD AMF",
	"vp9_amf":  "VP9 — AMD AMF",
	// QSV
	"h264_qsv": "H.264 — Intel Quick Sync",
	"hevc_qsv": "HEVC — Intel Quick Sync",
	"av1_qsv":  "AV1 — Intel Quick Sync",
	"vp9_qsv":  "VP9 — Intel Quick Sync",
	// VAAPI
	"h264_vaapi": "H.264 — VAAPI",
	"hevc_vaapi": "HEVC — VAAPI",
	"av1_vaapi":  "AV1 — VAAPI",
	"vp9_vaapi":  "VP9 — VAAPI",
}

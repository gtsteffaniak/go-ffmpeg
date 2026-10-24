package capabilities

import "strings"

// HWProbeCode classifies why a hardware encoder probe failed.
type HWProbeCode string

const (
	HWProbeOK                 HWProbeCode = ""
	HWProbeStaticBinary       HWProbeCode = "static_binary"
	HWProbeNoDevice           HWProbeCode = "no_device"
	HWProbeDevicePermission   HWProbeCode = "device_permission_denied"
	HWProbeLibvaMissing       HWProbeCode = "libva_missing"
	HWProbeVADriverMissing    HWProbeCode = "va_driver_missing"
	HWProbeVADriverInitFailed HWProbeCode = "va_driver_init_failed"
	HWProbeNvidiaLibsMissing  HWProbeCode = "nvidia_libs_missing"
	HWProbeNVENCUnsupported   HWProbeCode = "nvenc_unsupported_gpu"
	HWProbeNVENCSessionLimit  HWProbeCode = "nvenc_session_limit"
	HWProbeTimeout            HWProbeCode = "timeout"
	HWProbeSkipped            HWProbeCode = "skipped"
	HWProbeNotCompiled        HWProbeCode = "not_compiled"
	HWProbeUnknown            HWProbeCode = "unknown"
)

// EncoderProbeResult is the structured outcome of layered hardware detection.
type EncoderProbeResult struct {
	Encoder   string      `json:"encoder"`
	Accel     AccelType   `json:"accel"`
	Available bool        `json:"available"`
	Code      HWProbeCode `json:"code,omitempty"`
	Hint      string      `json:"hint,omitempty"`
	Detail    string      `json:"detail,omitempty"`
}

// HintForProbeCode returns a one-line remediation hint for a probe code.
func HintForProbeCode(code HWProbeCode) string {
	switch code {
	case HWProbeStaticBinary:
		return "use a glibc-linked or full dynamic ffmpeg build; static Linux binaries cannot load GPU userspace drivers (VAAPI/NVENC/QSV)"
	case HWProbeNoDevice:
		return "pass --device /dev/dri/renderD128 (or the correct render node) and ensure the GPU driver is loaded"
	case HWProbeDevicePermission:
		return "add the service user to the render group (gid from getent group render) and pass --device /dev/dri"
	case HWProbeLibvaMissing:
		return "install libva and VA drivers (e.g. intel-media-va-driver, mesa-va-drivers)"
	case HWProbeVADriverMissing:
		return "install a VA-API driver for your GPU (LIBVA_DRIVER_NAME / LIBVA_DRIVERS_PATH)"
	case HWProbeVADriverInitFailed:
		return "VA driver failed to initialize (often error -12): verify driver matches GPU, render node access, and non-static ffmpeg"
	case HWProbeNvidiaLibsMissing:
		return "install NVIDIA driver userspace libraries (libcuda.so.1, libnvidia-encode.so.1); in containers use --gpus all with NVIDIA Container Toolkit"
	case HWProbeNVENCUnsupported:
		return "GPU or driver does not support NVENC for this codec; update the driver or pick another encoder"
	case HWProbeNVENCSessionLimit:
		return "NVENC session limit reached; stop other encoders or reduce concurrent NVENC jobs"
	case HWProbeTimeout:
		return "hardware encode probe timed out; check driver health and reduce concurrent GPU load"
	case HWProbeNotCompiled:
		return "encoder is not compiled into this ffmpeg binary"
	case HWProbeSkipped:
		return "hardware probe skipped by configuration"
	default:
		return ""
	}
}

func accelForEncoderKind(kind string) AccelType {
	switch kind {
	case "nvenc":
		return AccelNVENC
	case "amf":
		return AccelAMF
	case "qsv":
		return AccelQSV
	case "vaapi":
		return AccelVAAPI
	case "videotoolbox":
		return AccelVideoToolbox
	case "v4l2m2m":
		return AccelV4L2M2M
	default:
		return AccelNone
	}
}

func formatProbeError(code HWProbeCode, hint, detail string) string {
	if code == HWProbeOK {
		return ""
	}
	msg := "[" + string(code) + "]"
	if hint != "" {
		msg += " " + hint
	}
	if detail != "" {
		msg += " (" + truncateProbeDetail(detail, 160) + ")"
	}
	return msg
}

func truncateProbeDetail(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

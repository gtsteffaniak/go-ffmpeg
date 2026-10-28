package capabilities

import "strings"

// HWProbeCode classifies why a hardware encoder probe failed or was skipped.
// Values are stable JSON strings for admin UIs and logs. Use [HintForProbeCode]
// for a one-line fix; [EncoderProbeResult.Hint] may add distro-specific text from
// [ApplyHostRemediation].
type HWProbeCode string

const (
	// HWProbeOK means the encoder passed layered detection (or was not probed).
	HWProbeOK HWProbeCode = ""

	// HWProbeStaticBinary means ffmpeg on Linux is statically linked and cannot
	// dlopen GPU userspace drivers. Fix: use a dynamic/glibc ffmpeg build.
	HWProbeStaticBinary HWProbeCode = "static_binary"

	// HWProbeNoDevice means no usable GPU device node was found (missing /dev/dri
	// render node, NVIDIA device, or WSL /dev/dxg). Fix: pass the correct device
	// and load the driver.
	HWProbeNoDevice HWProbeCode = "no_device"

	// HWProbeDevicePermission means a render node exists but cannot be opened
	// (EACCES). Fix: add the service user to the render group and pass --device /dev/dri.
	HWProbeDevicePermission HWProbeCode = "device_permission_denied"

	// HWProbeLibvaMissing means libva.so is not installed or not on the library path.
	// Fix: install libva and VA drivers for the GPU vendor.
	HWProbeLibvaMissing HWProbeCode = "libva_missing"

	// HWProbeVADriverMissing means no VA-API driver module was found for the GPU.
	// Fix: install mesa-va-drivers, intel-media-va-driver, or set LIBVA_DRIVERS_PATH.
	HWProbeVADriverMissing HWProbeCode = "va_driver_missing"

	// HWProbeVADriverInitFailed means libva loaded but the driver failed to initialize
	// (often VA error -12). Fix: match driver to GPU, check permissions, use dynamic ffmpeg.
	HWProbeVADriverInitFailed HWProbeCode = "va_driver_init_failed"

	// HWProbeNvidiaLibsMissing means libcuda.so.1 or libnvidia-encode.so.1 is missing.
	// Fix: install the NVIDIA driver stack; in containers use --gpus all and the NVIDIA toolkit.
	HWProbeNvidiaLibsMissing HWProbeCode = "nvidia_libs_missing"

	// HWProbeNVENCUnsupported means this GPU generation has no NVENC unit for the
	// codec (for example AV1 on Ampere). It is a hardware capability limit, not a
	// driver problem, so it is never promoted to [HWProbeGPUPresentDriverMissing].
	// Fix: use another hardware codec or the software encoder.
	HWProbeNVENCUnsupported HWProbeCode = "nvenc_unsupported_gpu"

	// HWProbeCodecUnsupported means the backend works (the driver stack loaded and
	// other codecs encode) but this GPU/driver has no hardware encoder for this
	// codec, for example AV1 encode on Intel Gen12 or Ampere. It is a hardware
	// capability limit and is never promoted to [HWProbeGPUPresentDriverMissing].
	HWProbeCodecUnsupported HWProbeCode = "hw_codec_unsupported"

	// HWProbeNVENCSessionLimit means NVENC concurrent session limit was hit during probe.
	// Fix: reduce concurrent NVENC jobs or close other encoders.
	HWProbeNVENCSessionLimit HWProbeCode = "nvenc_session_limit"

	// HWProbeTimeout means the functional encode probe exceeded its deadline.
	// Fix: check driver health and GPU load; increase detect timeout only if appropriate.
	HWProbeTimeout HWProbeCode = "timeout"

	// HWProbeSkipped means the functional probe was not run (SkipHWTests or equivalent).
	HWProbeSkipped HWProbeCode = "skipped"

	// HWProbeNotCompiled means ffmpeg does not include the encoder.
	// Fix: use a build with the required encoder enabled.
	HWProbeNotCompiled HWProbeCode = "not_compiled"

	// HWProbeGPUPresentDriverMissing means sysfs/PCI shows a GPU but the userspace stack
	// for the encoder backend is missing or unusable. Hint includes [HostRemediationMessage].
	HWProbeGPUPresentDriverMissing HWProbeCode = "gpu_present_driver_missing"

	// HWProbeVulkanLoaderMissing means libvulkan (or vulkan-1.dll on Windows) was not found.
	// Fix: install the Vulkan loader and mount GPU devices in containers.
	HWProbeVulkanLoaderMissing HWProbeCode = "vulkan_loader_missing"

	// HWProbeVulkanNoDevice means no Vulkan-capable device was available to ffmpeg.
	// Fix: pass render nodes or --gpus all with video capabilities on NVIDIA.
	HWProbeVulkanNoDevice HWProbeCode = "vulkan_no_device"

	// HWProbeVulkanNoVideoEncode means the driver/ICD lacks Vulkan video encode
	// queues (VK_KHR_video_encode_queue), including the ENOSYS / -38
	// "Function not implemented" failure. On WSL there is no NVIDIA Vulkan ICD, so
	// this is the expected result there.
	// Fix: upgrade Mesa RADV/ANV or the NVIDIA driver, or use NVENC/VAAPI.
	HWProbeVulkanNoVideoEncode HWProbeCode = "vulkan_no_video_encode_driver"

	// HWProbeVulkanDriverTooOld means the Vulkan driver API is too old for video encode.
	// Fix: upgrade the GPU driver.
	HWProbeVulkanDriverTooOld HWProbeCode = "vulkan_driver_too_old"

	// HWProbeUnknown means ffmpeg failed without a recognized pattern.
	// Fix: inspect [EncoderProbeResult.Detail] and driver logs.
	HWProbeUnknown HWProbeCode = "unknown"
)

// EncoderProbeResult is the structured outcome of layered hardware detection for one encoder.
// Code, Hint, and Detail are fixed after startup detection; they are not updated on transcode failure.
type EncoderProbeResult struct {
	// Encoder is the ffmpeg encoder name (for example h264_nvenc).
	Encoder string `json:"encoder"`
	// Accel is the backend family for reporting.
	Accel AccelType `json:"accel"`
	// Available is true only when the functional probe succeeded.
	Available bool `json:"available"`
	// Code classifies failure; empty when Available is true.
	Code HWProbeCode `json:"code,omitempty"`
	// OriginalCode is the classifier result before [ApplyHostRemediation] may
	// promote it; [demoteSiblingProbeFailures] only demotes originally-unknown
	// failures so genuine driver-init errors are never relabelled.
	OriginalCode HWProbeCode `json:"originalCode,omitempty"`
	// Hint is a short remediation line; may include distro guidance.
	Hint string `json:"hint,omitempty"`
	// Detail is truncated ffmpeg stderr from the probe, for operators.
	Detail string `json:"detail,omitempty"`
}

// HintForProbeCode returns a one-line remediation hint for code.
// It does not include distro package names; see [HostRemediationMessage] for those.
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
		return "this GPU does not support NVENC for this codec (hardware generation limit, not a driver update); use another hardware codec or the software encoder"
	case HWProbeCodecUnsupported:
		return "this GPU cannot hardware-encode this codec (hardware generation limit, not a missing driver); it may still decode it, and other codecs encode in hardware"
	case HWProbeNVENCSessionLimit:
		return "NVENC session limit reached; stop other encoders or reduce concurrent NVENC jobs"
	case HWProbeTimeout:
		return "hardware encode probe timed out; check driver health and reduce concurrent GPU load"
	case HWProbeNotCompiled:
		return "encoder is not compiled into this ffmpeg binary"
	case HWProbeSkipped:
		return "hardware probe skipped by configuration"
	case HWProbeGPUPresentDriverMissing:
		return "GPU hardware is present but the userspace driver/stack required for hardware encode is missing or unusable"
	case HWProbeVulkanLoaderMissing:
		return "install libvulkan (Vulkan loader); containers need the loader package and GPU device mounts"
	case HWProbeVulkanNoDevice:
		return "pass --device /dev/dri/renderD128 (or correct render node); Docker NVIDIA needs --gpus all with NVIDIA_DRIVER_CAPABILITIES=graphics,video,compute"
	case HWProbeVulkanNoVideoEncode:
		return "GPU driver lacks Vulkan video encode (VK_KHR_video_encode_queue); update Mesa RADV/ANV or NVIDIA driver"
	case HWProbeVulkanDriverTooOld:
		return "Vulkan driver is too old for FFmpeg video encode; upgrade Mesa (RADV/ANV) or NVIDIA driver"
	default:
		return ""
	}
}

// accelForEncoderKind maps internal encoder kind strings to [AccelType] for probe rows.
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
	case "vulkan":
		return AccelVulkan
	default:
		return AccelNone
	}
}

// formatProbeError builds the legacy TestError string from structured probe fields.
func formatProbeError(code HWProbeCode, hint, detail string) string {
	if code == HWProbeOK {
		return ""
	}
	msg := "[" + string(code) + "]"
	if hint != "" {
		msg += " " + hint
	}
	if detail != "" {
		msg += " (" + cutAtWord(detail, 160) + ")"
	}
	return msg
}

// truncateProbeDetail limits stderr embedded in hints and JSON.
func truncateProbeDetail(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

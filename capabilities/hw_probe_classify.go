package capabilities

import (
	"context"
	"strings"
)

// ClassifyHWProbeFailure maps ffmpeg stderr and context errors to an [HWProbeCode].
// It is used after the functional probe fails; prerequisite failures set codes earlier.
// Pass context.DeadlineExceeded in ctxErr for [HWProbeTimeout].
func ClassifyHWProbeFailure(stderr string, ctxErr error, encoderKind string) (HWProbeCode, string) {
	if ctxErr != nil && ctxErr == context.DeadlineExceeded {
		return HWProbeTimeout, HintForProbeCode(HWProbeTimeout)
	}
	lower := strings.ToLower(stderr)

	switch {
	case strings.Contains(lower, "not a dynamic executable") || strings.Contains(lower, "cannot load libnvidia-encode") && strings.Contains(lower, "static"):
		return HWProbeStaticBinary, HintForProbeCode(HWProbeStaticBinary)
	case strings.Contains(lower, "permission denied") && (strings.Contains(lower, "dri") || strings.Contains(lower, "render")):
		return HWProbeDevicePermission, HintForProbeCode(HWProbeDevicePermission)
	case strings.Contains(lower, "no va display found") || strings.Contains(lower, "no device available") && encoderKind == "vaapi":
		return HWProbeNoDevice, HintForProbeCode(HWProbeNoDevice)
	case strings.Contains(lower, "cannot load libva") || strings.Contains(lower, "failed to load libva"):
		return HWProbeLibvaMissing, HintForProbeCode(HWProbeLibvaMissing)
	case strings.Contains(lower, "failed to initialise vaapi") || strings.Contains(lower, "failed to initialize vaapi"):
		if strings.Contains(lower, "unknown libva error") || strings.Contains(lower, "error -12") || strings.Contains(lower, "error code: -12") {
			return HWProbeVADriverInitFailed, HintForProbeCode(HWProbeVADriverInitFailed)
		}
		return HWProbeVADriverInitFailed, HintForProbeCode(HWProbeVADriverInitFailed)
	case (encoderKind == "vaapi" || encoderKind == "qsv") &&
		strings.Contains(lower, "driver") && strings.Contains(lower, "not found") &&
		(strings.Contains(lower, "vaapi") || strings.Contains(lower, "libva") ||
			strings.Contains(lower, "va driver") || strings.Contains(lower, "_drv_video")):
		return HWProbeVADriverMissing, HintForProbeCode(HWProbeVADriverMissing)
	// The VA driver loaded and opened the device, but it exposes no encode
	// entrypoint for this codec profile (for example AV1 on Gen12). Hardware limit.
	case (encoderKind == "vaapi" || encoderKind == "") && strings.Contains(lower, "no usable encoding entrypoint"):
		return HWProbeCodecUnsupported, HintForProbeCode(HWProbeCodecUnsupported)
	// QSV reports an unsupported codec separately from parameter mismatches.
	// Parameter-only rejections stay neutral here; [demoteSiblingProbeFailures]
	// turns them into a codec limit only with evidence (a sibling probe with the
	// same parameters passed on the same backend).
	case encoderKind == "qsv" && strings.Contains(lower, "current codec type is unsupported"):
		return HWProbeCodecUnsupported, HintForProbeCode(HWProbeCodecUnsupported)
	case strings.Contains(lower, "cannot load libcuda") || strings.Contains(lower, "libnvidia-encode.so") && strings.Contains(lower, "cannot open shared object"):
		return HWProbeNvidiaLibsMissing, HintForProbeCode(HWProbeNvidiaLibsMissing)
	case strings.Contains(lower, "openencodesessionex failed") && strings.Contains(lower, "unsupported device"):
		return HWProbeNVENCUnsupported, HintForProbeCode(HWProbeNVENCUnsupported)
	// A GPU without an NVENC unit for the codec (for example AV1 on Ampere)
	// reports "Codec not supported" plus "No capable devices found". This is a
	// hardware capability limit, not a missing driver, so it must not be
	// promoted to gpu_present_driver_missing by [ApplyHostRemediation].
	case strings.Contains(lower, "no capable devices found") ||
		(strings.Contains(lower, "codec not supported") && encoderKind == "nvenc"):
		return HWProbeNVENCUnsupported, HintForProbeCode(HWProbeNVENCUnsupported)
	case strings.Contains(lower, "no nvenc capable devices") || strings.Contains(lower, "nvenc api version"):
		return HWProbeNVENCUnsupported, HintForProbeCode(HWProbeNVENCUnsupported)
	case strings.Contains(lower, "too many concurrent sessions") || strings.Contains(lower, "encoder sessions") && strings.Contains(lower, "exceeded"):
		return HWProbeNVENCSessionLimit, HintForProbeCode(HWProbeNVENCSessionLimit)
	case strings.Contains(lower, "unknown encoder"):
		return HWProbeNotCompiled, HintForProbeCode(HWProbeNotCompiled)
	case strings.Contains(lower, "libvulkan") && (strings.Contains(lower, "cannot open") || strings.Contains(lower, "failed to load")):
		return HWProbeVulkanLoaderMissing, HintForProbeCode(HWProbeVulkanLoaderMissing)
	// Vulkan video encode missing the codec extension is reported as ENOSYS
	// ("Function not implemented") / -38, or an explicit extension/queue message.
	// Treat it as "driver/ICD lacks Vulkan video encode", never as a missing
	// device or permission problem.
	case strings.Contains(lower, "vk_khr_video_encode") || strings.Contains(lower, "video encode queue") ||
		strings.Contains(lower, "function not implemented") || strings.Contains(lower, "error code: -38") ||
		strings.Contains(lower, "enosys"):
		if encoderKind == "vulkan" || strings.Contains(lower, "vulkan") {
			return HWProbeVulkanNoVideoEncode, HintForProbeCode(HWProbeVulkanNoVideoEncode)
		}
	case strings.Contains(lower, "vk_error") || strings.Contains(lower, "failed to create vulkan"):
		if strings.Contains(lower, "video") || strings.Contains(lower, "encode") {
			return HWProbeVulkanNoVideoEncode, HintForProbeCode(HWProbeVulkanNoVideoEncode)
		}
		return HWProbeVulkanNoDevice, HintForProbeCode(HWProbeVulkanNoDevice)
	case strings.Contains(lower, "no vulkan") || strings.Contains(lower, "vulkan device") && strings.Contains(lower, "not found"):
		return HWProbeVulkanNoDevice, HintForProbeCode(HWProbeVulkanNoDevice)
	case strings.Contains(lower, "video encode") && strings.Contains(lower, "not supported"):
		return HWProbeVulkanNoVideoEncode, HintForProbeCode(HWProbeVulkanNoVideoEncode)
	case strings.Contains(lower, "api version") && strings.Contains(lower, "vulkan") && strings.Contains(lower, "not supported"):
		return HWProbeVulkanDriverTooOld, HintForProbeCode(HWProbeVulkanDriverTooOld)
	case strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout"):
		return HWProbeTimeout, HintForProbeCode(HWProbeTimeout)
	}

	switch encoderKind {
	case "vaapi":
		if strings.Contains(lower, "error -12") || strings.Contains(lower, "error code: -12") {
			return HWProbeVADriverInitFailed, HintForProbeCode(HWProbeVADriverInitFailed)
		}
	case "nvenc":
		if strings.Contains(lower, "libnvidia-encode") {
			return HWProbeNvidiaLibsMissing, HintForProbeCode(HWProbeNvidiaLibsMissing)
		}
	case "vulkan":
		if strings.Contains(lower, "libvulkan") {
			return HWProbeVulkanLoaderMissing, HintForProbeCode(HWProbeVulkanLoaderMissing)
		}
		if strings.Contains(lower, "vk_khr_video_encode") || strings.Contains(lower, "video encode queue") {
			return HWProbeVulkanNoVideoEncode, HintForProbeCode(HWProbeVulkanNoVideoEncode)
		}
	}

	if stderr == "" {
		return HWProbeUnknown, HintForProbeCode(HWProbeUnknown)
	}
	return HWProbeUnknown, HintForProbeCode(HWProbeUnknown)
}

package capabilities

import (
	"context"
	"strings"
)

// ClassifyHWProbeFailure maps ffmpeg stderr/exit context to a probe code and hint.
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
	case strings.Contains(lower, "driver") && strings.Contains(lower, "not found") && strings.Contains(lower, "va"):
		return HWProbeVADriverMissing, HintForProbeCode(HWProbeVADriverMissing)
	case strings.Contains(lower, "cannot load libcuda") || strings.Contains(lower, "libnvidia-encode.so") && strings.Contains(lower, "cannot open shared object"):
		return HWProbeNvidiaLibsMissing, HintForProbeCode(HWProbeNvidiaLibsMissing)
	case strings.Contains(lower, "openencodesessionex failed") && strings.Contains(lower, "unsupported device"):
		return HWProbeNVENCUnsupported, HintForProbeCode(HWProbeNVENCUnsupported)
	case strings.Contains(lower, "no nvenc capable devices") || strings.Contains(lower, "nvenc api version"):
		return HWProbeNVENCUnsupported, HintForProbeCode(HWProbeNVENCUnsupported)
	case strings.Contains(lower, "too many concurrent sessions") || strings.Contains(lower, "encoder sessions") && strings.Contains(lower, "exceeded"):
		return HWProbeNVENCSessionLimit, HintForProbeCode(HWProbeNVENCSessionLimit)
	case strings.Contains(lower, "unknown encoder"):
		return HWProbeNotCompiled, HintForProbeCode(HWProbeNotCompiled)
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
	}

	if stderr == "" {
		return HWProbeUnknown, HintForProbeCode(HWProbeUnknown)
	}
	return HWProbeUnknown, HintForProbeCode(HWProbeUnknown)
}

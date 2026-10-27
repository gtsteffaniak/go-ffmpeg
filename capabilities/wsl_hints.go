package capabilities

import "github.com/gtsteffaniak/go-ffmpeg/platform"

// This file is the single source of WSL-specific hardware remediation text.
// Every startup hint that mentions WSL must route through [wslRemediation]
// (directly or via [wslPlatformRemediation]) so the wording cannot drift
// between emitters and cannot contradict the real host state (for example
// telling an operator to install a driver that is already working).
//
// WSL2 GPU paravirtualization exposes /dev/dxg and /usr/lib/wsl/lib but no
// /dev/dri render node. NVENC works through libnvidia-encode; there is no
// NVIDIA Vulkan ICD, so Vulkan video encode is unavailable and no render node
// is required for it either.

// wslRemediation returns WSL-aware remediation for a failed probe, or "" when
// the host is not WSL. It is answerable from platform gates alone so the
// prerequisite layer can use it before a [DetectionHost] exists.
func wslRemediation(wsl, dri, nvidia bool, encoderKind string, code HWProbeCode) string {
	if !wsl {
		return ""
	}
	switch encoderKind {
	case "vulkan":
		if nvidia {
			return "WSL: no NVIDIA Vulkan ICD is installed, so Vulkan video encode is unavailable; use NVENC (h264_nvenc/hevc_nvenc) instead"
		}
		return "WSL: Vulkan video encode is unavailable through GPU paravirtualization; use a vendor backend (NVENC/VAAPI) instead"
	case "nvenc":
		switch code {
		case HWProbeNvidiaLibsMissing:
			return "WSL: /usr/lib/wsl/lib is missing libcuda.so.1 or libnvidia-encode.so.1; repair the Windows NVIDIA driver"
		case HWProbeNoDevice:
			return "WSL: /dev/dxg is missing; enable GPU paravirtualization (wsl --update, then restart WSL)"
		case HWProbeGPUPresentDriverMissing:
			return "WSL: /dev/dxg is present but the NVIDIA userspace libraries are unavailable; check /usr/lib/wsl/lib is on the loader path"
		}
	case "vaapi", "qsv":
		if !dri {
			return "WSL: no /dev/dri render node (expected under GPU paravirtualization); VAAPI/QSV are unavailable, use NVENC or software"
		}
	}
	return ""
}

// wslPlatformRemediation builds the prerequisite-layer WSL hint from platform gates.
func wslPlatformRemediation(plat platform.Info, encoderKind string, code HWProbeCode) string {
	return wslRemediation(plat.WSL, plat.DRI, plat.NVIDIA, encoderKind, code)
}

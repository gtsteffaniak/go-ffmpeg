package capabilities

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// hwTroubleshoot returns a primary message and optional follow-up hints for report output.
func hwTroubleshoot(encoderName, kind, output string, plat platform.Info) (primary string, hints []string) {
	output = strings.ToLower(output)

	switch kind {
	case "qsv":
		if strings.Contains(encoderName, "vp9") && qsvVP9EncodeUnsupported(output) {
			primary = vp9IntelEncodePrimary(plat)
			hints = vp9IntelEncodeHints(plat)
			return primary, hints
		}
		if strings.Contains(output, "error creating a mfx session") || strings.Contains(output, "mfx_err") {
			if !plat.QSVRuntime {
				runtimePkg := qsvRuntimePackageLabel(plat)
				primary = "[driver] Intel oneVPL GPU runtime missing — MFX -9 from libvpl2 without " + runtimePkg
			} else {
				primary = "[runtime] Intel Quick Sync session failed (MFX -9) — GPU runtime present but session init failed"
			}
			hints = qsvHints(encoderName, plat)
			return primary, hints
		}
	case "vaapi":
		if strings.Contains(encoderName, "vp9") && (vaapiVP9EncodeUnsupported(output) || vaapiHardwareLimit(output)) {
			primary = vp9IntelEncodePrimary(plat)
			hints = vp9IntelEncodeHints(plat)
			return primary, hints
		}
		if vaapiHardwareLimit(output) {
			primary = fmt.Sprintf("[hardware] %s encode not supported by this GPU's VAAPI driver", vaapiCodecName(encoderName))
			hints = []string{
				"Not an OS or FFmpeg compile issue — other VAAPI codecs may still work",
				"Use the software encoder listed under the same codec section above",
			}
			return primary, hints
		}
		if strings.Contains(output, "failed to initialise vaapi") || strings.Contains(output, "cannot load libva") {
			primary = "[driver] VAAPI initialization failed"
			hints = vaapiDriverHints(plat)
			return primary, hints
		}
		if strings.Contains(output, "permission denied") {
			primary = "[permission] Cannot access GPU render node"
			hints = []string{"Fix: sudo usermod -aG render,video $USER then log out/in"}
			return primary, hints
		}
	case "vulkan":
		if strings.Contains(output, "libvulkan") {
			primary = "[driver] Vulkan loader missing or not loadable"
			hints = []string{"Fix: install libvulkan1 / vulkan-loader; in Docker pass GPU devices and driver capabilities"}
			return primary, hints
		}
		if strings.Contains(output, "vk_khr_video_encode") || strings.Contains(output, "video encode queue") ||
			strings.Contains(output, "function not implemented") || strings.Contains(output, "error code: -38") ||
			strings.Contains(output, "video encode") {
			primary = "[hardware] Vulkan video encode not supported by this GPU driver/ICD"
			if wsl := wslPlatformRemediation(plat, "vulkan", HWProbeVulkanNoVideoEncode); wsl != "" {
				hints = []string{
					wsl,
					"Vendor paths (NVENC/VAAPI/QSV) still work — Vulkan is optional fallback",
				}
				return primary, hints
			}
			hints = []string{
				"Update Mesa RADV/ANV (24.2+) or NVIDIA driver (550+) for VK_KHR_video_encode_queue",
				"Vendor paths (VAAPI/NVENC/QSV) may still work — Vulkan is optional fallback",
			}
			return primary, hints
		}
	case "nvenc":
		if strings.Contains(output, "unknown encoder") {
			primary = "[compile] Encoder not built into this FFmpeg binary"
			hints = []string{"Fix: install a full FFmpeg build or rebuild with --enable-nvenc"}
			return primary, hints
		}
	}

	primary = summarizeHWError(output)
	return primary, nil
}

func qsvRuntimePackageLabel(plat platform.Info) string {
	pkgs := platform.NewPackageHints(plat.Distro).Distro.QSVRuntimePackages()
	if len(pkgs) > 0 {
		return pkgs[0]
	}
	return "libmfx-gen"
}

func qsvHints(encoderName string, plat platform.Info) []string {
	if !plat.QSVRuntime {
		return platform.NewPackageHints(plat.Distro).QSVRuntimeFix()
	}

	hints := []string{
		"Runtime installed but session failed — check driver/runtime version alignment",
		"Verify: vainfo lists encode profiles; Intel VAAPI driver and oneVPL GPU runtime versions should match",
		"Check: iGPU enabled in BIOS; user in render/video group (groups | grep render)",
		"Ubuntu FFmpeg uses oneVPL (--enable-libvpl), not legacy libmfx1",
	}
	if sibling := vaapiSiblingEncoder(encoderName); sibling != "" && plat.VAAPI {
		hints = append(hints, fmt.Sprintf("Workaround: %s works on this GPU if QSV remains broken after installing libmfx-gen1.2", sibling))
	}
	return hints
}

func vaapiDriverHints(plat platform.Info) []string {
	return platform.NewPackageHints(plat.Distro).VAAPIInitFix(plat.Intel)
}

func qsvVP9EncodeUnsupported(output string) bool {
	patterns := []string{
		"current pixel format is unsupported",
		"not supported by the qsv runtime",
		"error code: -22",
		"function not implemented",
		"invalid argument",
	}
	for _, p := range patterns {
		if strings.Contains(output, p) {
			return true
		}
	}
	return false
}

func vaapiVP9EncodeUnsupported(output string) bool {
	patterns := []string{
		"error code: -22",
		"function not implemented",
		"no usable encoding entrypoint",
		"profilevpp9",
	}
	for _, p := range patterns {
		if strings.Contains(output, p) {
			return true
		}
	}
	return false
}

func vp9IntelEncodePrimary(plat platform.Info) string {
	if plat.OS == "linux" && plat.Intel {
		return "[hardware] VP9 hw encode unavailable on Linux (Intel driver is decode-only; Windows may support encode)"
	}
	return "[hardware] VP9 hardware encode not supported by this GPU's driver stack"
}

func vp9IntelEncodeHints(_ platform.Info) []string {
	return []string{
		"Use Software — libvpx above for encode; QSV/VAAPI hw decode still works",
	}
}

func vaapiHardwareLimit(output string) bool {
	patterns := []string{
		"no usable encoding entrypoint",
		"function not implemented",
		"profilevpp9",
		"vaapi_encode",
		"does not support encoding",
	}
	for _, p := range patterns {
		if strings.Contains(output, p) {
			return true
		}
	}
	return false
}

func vaapiCodecName(encoderName string) string {
	switch {
	case strings.Contains(encoderName, "vp9"):
		return "VP9"
	case strings.Contains(encoderName, "av1"):
		return "AV1"
	case strings.Contains(encoderName, "hevc"):
		return "HEVC"
	case strings.Contains(encoderName, "h264"):
		return "H.264"
	default:
		return "This codec"
	}
}

func vaapiSiblingEncoder(qsvName string) string {
	switch qsvName {
	case "h264_qsv":
		return "h264_vaapi"
	case "hevc_qsv":
		return "hevc_vaapi"
	case "av1_qsv":
		return "av1_vaapi"
	default:
		return ""
	}
}

func formatTestMessages(primary string, hints []string) string {
	if primary == "" {
		return ""
	}
	if len(hints) == 0 {
		return primary
	}
	parts := []string{primary}
	parts = append(parts, hints...)
	return strings.Join(parts, "\n")
}

package capabilities

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// DetectionHost captures distro/container/GPU presence for actionable probe hints.
type DetectionHost struct {
	Presence  platform.GPUPresence   `json:"gpuPresence,omitempty"`
	Distro    platform.DistroContext `json:"distro,omitempty"`
	Container bool                   `json:"container,omitempty"`
}

// NewDetectionHost probes the host once (independent of ffmpeg drivers).
func NewDetectionHost() DetectionHost {
	return DetectionHost{
		Presence:  platform.DetectGPUPresence(),
		Distro:    platform.DetectDistro(),
		Container: platform.RunningInContainer(),
	}
}

func probeCodeDriverRelated(code HWProbeCode) bool {
	switch code {
	case HWProbeStaticBinary, HWProbeNoDevice, HWProbeDevicePermission,
		HWProbeLibvaMissing, HWProbeVADriverMissing, HWProbeVADriverInitFailed,
		HWProbeNvidiaLibsMissing, HWProbeVulkanLoaderMissing, HWProbeVulkanNoDevice,
		HWProbeVulkanNoVideoEncode, HWProbeVulkanDriverTooOld, HWProbeUnknown:
		return true
	default:
		return false
	}
}

func vendorForEncoderKind(kind string, presence platform.GPUPresence) string {
	switch kind {
	case "nvenc":
		if presence.NVIDIA {
			return "nvidia"
		}
	case "amf", "vaapi":
		if presence.AMD {
			return "amd"
		}
		if presence.Intel {
			return "intel"
		}
	case "qsv":
		if presence.Intel {
			return "intel"
		}
	case "vulkan":
		if presence.NVIDIA {
			return "nvidia"
		}
		if presence.AMD {
			return "amd"
		}
		if presence.Intel {
			return "intel"
		}
	case "v4l2m2m":
		return ""
	}
	if presence.NVIDIA {
		return "nvidia"
	}
	if presence.Intel {
		return "intel"
	}
	if presence.AMD {
		return "amd"
	}
	return ""
}

func gpuPresentForVendor(vendor string, presence platform.GPUPresence) bool {
	switch vendor {
	case "intel":
		return presence.Intel
	case "amd":
		return presence.AMD
	case "nvidia":
		return presence.NVIDIA
	default:
		return false
	}
}

// ApplyHostRemediation augments failed probe codes/hints when hardware exists but drivers/stack are missing.
func ApplyHostRemediation(host DetectionHost, encoderKind string, binary FFmpegBinaryInfo, code HWProbeCode, hint string) (HWProbeCode, string) {
	if code == HWProbeOK || code == HWProbeSkipped || code == HWProbeNotCompiled {
		return code, hint
	}
	vendor := vendorForEncoderKind(encoderKind, host.Presence)
	if vendor == "" || !gpuPresentForVendor(vendor, host.Presence) {
		return code, hint
	}
	if !probeCodeDriverRelated(code) {
		return code, hint
	}
	remediation := HostRemediationMessage(host, vendor, encoderKind, binary)
	if remediation == "" {
		return code, hint
	}
	newCode := HWProbeGPUPresentDriverMissing
	if hint != "" && !strings.Contains(hint, remediation) {
		hint = hint + " | " + remediation
	} else {
		hint = remediation
	}
	return newCode, hint
}

// HostRemediationMessage returns vendor+distro-specific install guidance (informational).
func HostRemediationMessage(host DetectionHost, vendor, encoderKind string, binary FFmpegBinaryInfo) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("GPU detected (%s) but %s encode is unavailable on this host", vendorLabel(vendor), encoderBackendLabel(encoderKind)))

	if binary.Kind == FFmpegLinkStatic {
		parts = append(parts, "use a full dynamic/glibc ffmpeg build (static binaries cannot load GPU userspace drivers)")
	}

	switch vendor {
	case "intel":
		pkgs := host.Distro.IntelVADriverPackages()
		if len(pkgs) > 0 {
			parts = append(parts, "install Intel VAAPI/QSV stack: "+host.Distro.InstallCommand(pkgs...))
		}
		if encoderKind == "qsv" {
			parts = append(parts, host.Distro.QSVRuntimeHint())
		}
	case "amd":
		pkgs := host.Distro.AMDVADriverPackages()
		if len(pkgs) > 0 {
			msg := "install AMD VAAPI encode: " + host.Distro.InstallCommand(pkgs...)
			if host.Distro.Family == platform.DistroFamilyRPM {
				msg += " (Fedora: enable RPM Fusion; stock Mesa may not expose H.264/HEVC encode)"
			}
			parts = append(parts, msg)
		}
	case "nvidia":
		if host.Container {
			parts = append(parts, "Docker: run with --gpus all and NVIDIA Container Toolkit; use a non-static ffmpeg image with libnvidia-encode")
		} else if host.Presence.WSLDXG {
			parts = append(parts, "WSL: ensure Windows NVIDIA driver is current and /usr/lib/wsl/lib is mounted")
		} else {
			parts = append(parts, "install the proprietary NVIDIA driver including libnvidia-encode.so.1 and libcuda.so.1")
		}
	}

	if host.Container && vendor != "nvidia" {
		parts = append(parts, "container: pass --device /dev/dri (all render nodes) and add the container user to the host render group")
	}

	return strings.Join(parts, "; ")
}

func vendorLabel(v string) string {
	switch v {
	case "intel":
		return "Intel"
	case "amd":
		return "AMD"
	case "nvidia":
		return "NVIDIA"
	default:
		return v
	}
}

func encoderBackendLabel(kind string) string {
	switch kind {
	case "nvenc":
		return "NVENC"
	case "qsv":
		return "QSV"
	case "vaapi":
		return "VAAPI"
	case "amf":
		return "AMF"
	case "vulkan":
		return "Vulkan"
	default:
		return kind
	}
}

// GPUPresentRemediationLogLines returns deduplicated startup log lines for failed HW probes when a GPU is present.
func (c *Capabilities) GPUPresentRemediationLogLines() []string {
	if c == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var lines []string
	for _, probe := range c.EncoderProbeResults() {
		if probe.Available || probe.Code != HWProbeGPUPresentDriverMissing {
			continue
		}
		line := probe.Encoder + ": " + probe.Hint
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, "hardware probe: "+line)
	}
	if len(lines) == 0 {
		for vendor, msg := range c.vendorLevelRemediation() {
			line := vendor + ": " + msg
			if _, ok := seen[line]; ok {
				continue
			}
			seen[line] = struct{}{}
			lines = append(lines, "hardware probe: "+line)
		}
	}
	return lines
}

func (c *Capabilities) vendorLevelRemediation() map[string]string {
	out := map[string]string{}
	host := c.DetectionHost
	if !host.Presence.Any() {
		return out
	}
	kinds := map[string]string{
		"nvenc": "nvidia", "qsv": "intel", "vaapi": "", "amf": "amd", "vulkan": "",
	}
	for _, known := range KnownEncoders {
		if !known.HW {
			continue
		}
		enc := c.Encoders[known.Name]
		if enc.Available || !enc.Tested {
			continue
		}
		if !probeCodeDriverRelated(enc.ProbeCode) {
			continue
		}
		vendor := kinds[known.Kind]
		if vendor == "" {
			vendor = vendorForEncoderKind(known.Kind, host.Presence)
		}
		if vendor == "" || !gpuPresentForVendor(vendor, host.Presence) {
			continue
		}
		if _, ok := out[vendor]; ok {
			continue
		}
		msg := HostRemediationMessage(host, vendor, known.Kind, c.FFmpegBinary)
		if msg != "" {
			out[vendor] = msg
		}
	}
	return out
}

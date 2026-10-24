package capabilities

import "strings"

// VulkanProbeWarnLines returns startup warn lines for failed Vulkan encoder probes.
func (c *Capabilities) VulkanProbeWarnLines() []string {
	if c == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var lines []string
	for _, probe := range c.EncoderProbeResults() {
		if probe.Accel != AccelVulkan || probe.Available {
			continue
		}
		if !vulkanProbeWarnCode(probe.Code) {
			continue
		}
		line := probe.Encoder + ": [" + string(probe.Code) + "] " + probe.Hint
		if extra := vulkanVendorRemediation(c.DetectionHost, probe.Code); extra != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(extra)) {
			line += " | " + extra
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, "vulkan probe: "+line)
	}
	return lines
}

func vulkanProbeWarnCode(code HWProbeCode) bool {
	switch code {
	case HWProbeVulkanLoaderMissing, HWProbeVulkanNoDevice, HWProbeVulkanNoVideoEncode,
		HWProbeVulkanDriverTooOld, HWProbeStaticBinary, HWProbeGPUPresentDriverMissing, HWProbeUnknown:
		return true
	default:
		return code != HWProbeOK && code != HWProbeSkipped && code != HWProbeNotCompiled
	}
}

func vulkanVendorRemediation(host DetectionHost, code HWProbeCode) string {
	if !host.Presence.Any() {
		return ""
	}
	switch code {
	case HWProbeVulkanLoaderMissing:
		if host.Container {
			return "containers: install vulkan-loader package and pass GPU devices (--device /dev/dri or --gpus all)"
		}
	case HWProbeVulkanNoDevice:
		if host.Container {
			return "Docker: --device /dev/dri/renderD128; NVIDIA: --gpus all -e NVIDIA_DRIVER_CAPABILITIES=graphics,video,compute"
		}
	case HWProbeVulkanNoVideoEncode, HWProbeVulkanDriverTooOld:
		if host.Presence.AMD {
			return "AMD: Mesa RADV 24.2+ with VK_KHR_video_encode_queue; Fedora: mesa-va-drivers-freeworld + recent mesa-vulkan-drivers"
		}
		if host.Presence.Intel {
			return "Intel: Mesa ANV 24.2+ or intel-media-driver stack with Vulkan video encode support"
		}
		if host.Presence.NVIDIA {
			return "NVIDIA: driver 550+ with Vulkan video encode; containers need graphics,video capabilities"
		}
	}
	return ""
}

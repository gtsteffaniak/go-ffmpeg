package capabilities

// VulkanProbeWarnLines returns startup warn lines for failed Vulkan encoder
// probes. It is a filtered view of [Capabilities.StartupHardwareWarnLines], so
// calling it alongside [Capabilities.GPUPresentRemediationLogLines] never emits
// the same remediation twice.
//
// Prefer [Capabilities.StartupHardwareWarnLines], which merges both emitters.
func (c *Capabilities) VulkanProbeWarnLines() []string {
	return c.warnLinesOfKind(warnKindVulkan)
}

func vulkanProbeWarnCode(code HWProbeCode) bool {
	switch code {
	case HWProbeVulkanLoaderMissing, HWProbeVulkanNoDevice, HWProbeVulkanNoVideoEncode,
		HWProbeVulkanDriverTooOld, HWProbeStaticBinary, HWProbeUnknown:
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
		if wsl := wslRemediation(host.WSL, host.DRI, host.Presence.NVIDIA, "vulkan", code); wsl != "" {
			return wsl
		}
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

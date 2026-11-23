package capabilities

import (
	"fmt"
	"strings"
)

// VulkanEncoderMode controls whether Vulkan is included in the encoder hierarchy.
type VulkanEncoderMode string

const (
	// VulkanEncoderOff disables Vulkan in the encoder hierarchy (explicit opt-out).
	VulkanEncoderOff VulkanEncoderMode = "off"
	// VulkanEncoderAuto (empty config) appends Vulkan after detection when an encode probe passed.
	VulkanEncoderAuto     VulkanEncoderMode = ""
	VulkanEncoderFallback VulkanEncoderMode = "fallback"
)

// ParseVulkanEncoderMode validates config for Vulkan backend selection.
func ParseVulkanEncoderMode(raw string) (VulkanEncoderMode, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "off", "false", "disabled", "none":
		return VulkanEncoderOff, nil
	case "", "default", "auto":
		return VulkanEncoderAuto, nil
	case "fallback", "lowest", "on", "enabled", "true":
		return VulkanEncoderFallback, nil
	default:
		return VulkanEncoderOff, fmt.Errorf("unknown vulkan encoder mode %q (use off or fallback)", raw)
	}
}

// MergeVulkanHierarchy appends Vulkan as lowest-priority HW when mode is fallback,
// unless the caller set an explicit hierarchy that already includes or excludes Vulkan.
func MergeVulkanHierarchy(hierarchy []AccelType, explicit []AccelType, mode VulkanEncoderMode) []AccelType {
	if len(explicit) > 0 {
		return hierarchy
	}
	if mode != VulkanEncoderFallback {
		return hierarchy
	}
	for _, a := range hierarchy {
		if a == AccelVulkan {
			return hierarchy
		}
	}
	out := append([]AccelType(nil), hierarchy...)
	out = append(out, AccelVulkan)
	return out
}

// AppendWorkingVulkan adds AccelVulkan after vendor backends when any Vulkan
// encoder passed functional detection and the caller did not opt out.
func AppendWorkingVulkan(hierarchy []AccelType, explicit []AccelType, mode VulkanEncoderMode, caps *Capabilities) []AccelType {
	if len(explicit) > 0 || mode == VulkanEncoderOff || caps == nil {
		return hierarchy
	}
	if mode == VulkanEncoderFallback {
		return hierarchy
	}
	for _, a := range hierarchy {
		if a == AccelVulkan {
			return hierarchy
		}
	}
	for _, known := range KnownEncoders {
		if !known.HW || known.Kind != "vulkan" {
			continue
		}
		if enc, ok := caps.Encoders[known.Name]; ok && enc.Available {
			return insertVulkanBeforeAccelNone(hierarchy)
		}
	}
	return hierarchy
}

// insertVulkanBeforeAccelNone appends Vulkan after vendor backends and before software.
func insertVulkanBeforeAccelNone(hierarchy []AccelType) []AccelType {
	for _, a := range hierarchy {
		if a == AccelVulkan {
			return hierarchy
		}
	}
	out := make([]AccelType, 0, len(hierarchy)+1)
	for _, a := range hierarchy {
		if a == AccelNone {
			out = append(out, AccelVulkan)
		}
		if a == AccelVulkan {
			continue
		}
		out = append(out, a)
	}
	if len(out) == len(hierarchy) {
		out = append(out, AccelVulkan)
	}
	return out
}

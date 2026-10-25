package capabilities

import (
	"fmt"
	"strings"
)

// VulkanEncoderMode controls whether Vulkan is included in the encoder hierarchy.
type VulkanEncoderMode string

const (
	VulkanEncoderOff      VulkanEncoderMode = ""
	VulkanEncoderFallback VulkanEncoderMode = "fallback"
)

// ParseVulkanEncoderMode validates config for Vulkan backend selection.
func ParseVulkanEncoderMode(raw string) (VulkanEncoderMode, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "", "off", "false", "disabled", "none":
		return VulkanEncoderOff, nil
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

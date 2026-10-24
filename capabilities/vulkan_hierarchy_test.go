package capabilities_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestMergeVulkanHierarchyFallback(t *testing.T) {
	base := []capabilities.AccelType{capabilities.AccelVAAPI, capabilities.AccelNVENC}
	got := capabilities.MergeVulkanHierarchy(base, nil, capabilities.VulkanEncoderFallback)
	want := []capabilities.AccelType{capabilities.AccelVAAPI, capabilities.AccelNVENC, capabilities.AccelVulkan}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestMergeVulkanHierarchyExplicitWins(t *testing.T) {
	explicit := []capabilities.AccelType{capabilities.AccelNVENC}
	base := capabilities.MergeVulkanHierarchy(explicit, explicit, capabilities.VulkanEncoderFallback)
	if len(base) != 1 || base[0] != capabilities.AccelNVENC {
		t.Fatalf("explicit hierarchy should not auto-append vulkan: %v", base)
	}
}

func TestParseVulkanEncoderMode(t *testing.T) {
	mode, err := capabilities.ParseVulkanEncoderMode("fallback")
	if err != nil || mode != capabilities.VulkanEncoderFallback {
		t.Fatalf("mode=%v err=%v", mode, err)
	}
	mode, err = capabilities.ParseVulkanEncoderMode("off")
	if err != nil || mode != capabilities.VulkanEncoderOff {
		t.Fatalf("mode=%v err=%v", mode, err)
	}
}

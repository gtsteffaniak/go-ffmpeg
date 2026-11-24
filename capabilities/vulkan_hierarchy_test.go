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
	mode, err = capabilities.ParseVulkanEncoderMode("")
	if err != nil || mode != capabilities.VulkanEncoderAuto {
		t.Fatalf("empty mode=%v err=%v want auto", mode, err)
	}
	mode, err = capabilities.ParseVulkanEncoderMode("on")
	if err != nil || mode != capabilities.VulkanEncoderFallback {
		t.Fatalf("on mode=%v err=%v want fallback", mode, err)
	}
	mode, err = capabilities.ParseVulkanEncoderMode("auto")
	if err != nil || mode != capabilities.VulkanEncoderAuto {
		t.Fatalf("auto mode=%v err=%v want auto", mode, err)
	}
}

func TestMergeVulkanHierarchyInsertsBeforeAccelNone(t *testing.T) {
	base := []capabilities.AccelType{capabilities.AccelNone}
	got := capabilities.MergeVulkanHierarchy(base, nil, capabilities.VulkanEncoderFallback)
	want := []capabilities.AccelType{capabilities.AccelVulkan, capabilities.AccelNone}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestAppendWorkingVulkanWhenProbePassed(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["h264_vulkan"] = capabilities.EncoderCapability{Name: "h264_vulkan", Kind: "vulkan", Available: true, Tested: true}
	base := []capabilities.AccelType{capabilities.AccelNVENC}
	got := capabilities.AppendWorkingVulkan(base, nil, capabilities.VulkanEncoderAuto, caps)
	if len(got) != 2 || got[1] != capabilities.AccelVulkan {
		t.Fatalf("got %v", got)
	}
}

func TestAppendWorkingVulkanBeforeAccelNoneSoftwareOnly(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["h264_vulkan"] = capabilities.EncoderCapability{Name: "h264_vulkan", Kind: "vulkan", Available: true, Tested: true}
	// Helper behavior with an empty explicit hierarchy: Auto still inserts
	// Vulkan before software. Detect treats a software-only GPU choice as
	// explicit in Auto, so this path is not used for "CPU (software)".
	base := []capabilities.AccelType{capabilities.AccelNone}
	got := capabilities.AppendWorkingVulkan(base, nil, capabilities.VulkanEncoderAuto, caps)
	want := []capabilities.AccelType{capabilities.AccelVulkan, capabilities.AccelNone}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestAppendWorkingVulkanExplicitAccelNoneBlocks(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["h264_vulkan"] = capabilities.EncoderCapability{Name: "h264_vulkan", Kind: "vulkan", Available: true, Tested: true}
	base := []capabilities.AccelType{capabilities.AccelNone}
	got := capabilities.AppendWorkingVulkan(base, []capabilities.AccelType{capabilities.AccelNone}, capabilities.VulkanEncoderAuto, caps)
	if len(got) != 1 || got[0] != capabilities.AccelNone {
		t.Fatalf("got %v", got)
	}
}

func TestAppendWorkingVulkanRespectsOff(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["h264_vulkan"] = capabilities.EncoderCapability{Name: "h264_vulkan", Kind: "vulkan", Available: true, Tested: true}
	base := []capabilities.AccelType{capabilities.AccelNVENC}
	got := capabilities.AppendWorkingVulkan(base, nil, capabilities.VulkanEncoderOff, caps)
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}

package capabilities

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestDeviceChainPrefersVulkanBeforeSoftware(t *testing.T) {
	caps := NewCapabilities()
	caps.AllowSoftware = true
	caps.Encoders["h264_nvenc"] = EncoderCapability{Name: "h264_nvenc", Kind: "nvenc", Available: false, Tested: true, Compiled: true}
	caps.Encoders["h264_vulkan"] = EncoderCapability{Name: "h264_vulkan", Kind: "vulkan", Available: true, Tested: true, Compiled: true}
	caps.Encoders["libx264"] = EncoderCapability{Name: "libx264", Kind: "software", Available: true, Compiled: true}
	caps.CodecMatrix[CodecH264] = CodecSupport{
		Hardware: map[AccelType]string{AccelNVENC: "h264_nvenc", AccelVulkan: "h264_vulkan"},
		Software: []string{"libx264"},
	}
	hierarchy := []AccelType{AccelNVENC, AccelVulkan, AccelNone}
	chain := []platform.GPUChoice{
		{Enabled: true, Vendor: "nvidia", Name: "NVIDIA GeForce RTX 3050"},
		{SoftwareOnly: true, Name: "CPU (software)"},
	}
	if err := ApplyDeviceSelection(caps, hierarchy, chain, chain[0]); err != nil {
		t.Fatal(err)
	}
	sel := caps.CodecMatrix[CodecH264].Preferred
	if sel.Encoder != "h264_vulkan" || sel.Accel != AccelVulkan {
		t.Fatalf("preferred %+v want h264_vulkan/vulkan", sel)
	}
}

func TestDeviceChainRecordsVulkanFailureAttempt(t *testing.T) {
	caps := NewCapabilities()
	caps.AllowSoftware = true
	caps.Encoders["h264_vulkan"] = EncoderCapability{
		Name: "h264_vulkan", Kind: "vulkan", Available: false, Tested: true, Compiled: true,
		TestError: "[vulkan_no_video_encode_driver] missing",
	}
	caps.Encoders["libx264"] = EncoderCapability{Name: "libx264", Kind: "software", Available: true, Compiled: true}
	caps.CodecMatrix[CodecH264] = CodecSupport{
		Hardware: map[AccelType]string{AccelVulkan: "h264_vulkan"},
		Software: []string{"libx264"},
	}
	hierarchy := []AccelType{AccelVulkan, AccelNone}
	chain := []platform.GPUChoice{{SoftwareOnly: true, Name: "CPU (software)"}}
	if err := ApplyDeviceSelection(caps, hierarchy, chain, chain[0]); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, att := range caps.DeviceAttempts {
		if att.Encoder == "h264_vulkan" && att.Reason != "" && !att.Selected {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("DeviceAttempts=%+v", caps.DeviceAttempts)
	}
}

func TestSoftwareOnlyBranchSkipsVulkanWhenNotInHierarchy(t *testing.T) {
	caps := NewCapabilities()
	caps.AllowSoftware = true
	caps.Encoders["h264_vulkan"] = EncoderCapability{Name: "h264_vulkan", Kind: "vulkan", Available: true, Tested: true, Compiled: true}
	caps.Encoders["libx264"] = EncoderCapability{Name: "libx264", Kind: "software", Available: true, Compiled: true}
	caps.CodecMatrix[CodecH264] = CodecSupport{
		Hardware: map[AccelType]string{AccelVulkan: "h264_vulkan"},
		Software: []string{"libx264"},
	}
	hierarchy := []AccelType{AccelNone}
	chain := []platform.GPUChoice{{SoftwareOnly: true, Name: "CPU (software)"}}
	if err := ApplyDeviceSelection(caps, hierarchy, chain, chain[0]); err != nil {
		t.Fatal(err)
	}
	sel := caps.CodecMatrix[CodecH264].Preferred
	if sel.Encoder != "libx264" || sel.Accel != AccelNone {
		t.Fatalf("preferred %+v want libx264/none", sel)
	}
	for _, att := range caps.DeviceAttempts {
		if att.Encoder == "h264_vulkan" {
			t.Fatalf("software-only Auto must not attempt Vulkan: %+v", caps.DeviceAttempts)
		}
	}
}

func TestSoftwareOnlyBranchUsesVulkanWhenHierarchyIncludesIt(t *testing.T) {
	caps := NewCapabilities()
	caps.AllowSoftware = true
	caps.Encoders["h264_vulkan"] = EncoderCapability{Name: "h264_vulkan", Kind: "vulkan", Available: true, Tested: true, Compiled: true}
	caps.Encoders["libx264"] = EncoderCapability{Name: "libx264", Kind: "software", Available: true, Compiled: true}
	caps.CodecMatrix[CodecH264] = CodecSupport{
		Hardware: map[AccelType]string{AccelVulkan: "h264_vulkan"},
		Software: []string{"libx264"},
	}
	hierarchy := []AccelType{AccelVulkan, AccelNone}
	chain := []platform.GPUChoice{{SoftwareOnly: true, Name: "CPU (software)"}}
	if err := ApplyDeviceSelection(caps, hierarchy, chain, chain[0]); err != nil {
		t.Fatal(err)
	}
	sel := caps.CodecMatrix[CodecH264].Preferred
	if sel.Encoder != "h264_vulkan" || sel.Accel != AccelVulkan {
		t.Fatalf("preferred %+v want h264_vulkan/vulkan", sel)
	}
}

package capabilities_test

import (
	"context"
	"os"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func softwareOnlyGPU() platform.GPUChoice {
	return platform.GPUChoice{Mode: platform.GPUModeSoftware, SoftwareOnly: true, Enabled: true, Name: "CPU (software)"}
}

func hierarchyHas(h []capabilities.AccelType, want capabilities.AccelType) bool {
	for _, a := range h {
		if a == want {
			return true
		}
	}
	return false
}

func vulkanBeforeSoftware(h []capabilities.AccelType) bool {
	var hasVulkan, hasSoftware, orderOK bool
	orderOK = true
	for i, a := range h {
		switch a {
		case capabilities.AccelVulkan:
			hasVulkan = true
			if i+1 < len(h) && h[i+1] != capabilities.AccelNone {
				orderOK = false
			}
		case capabilities.AccelNone:
			hasSoftware = true
			if i > 0 && h[i-1] != capabilities.AccelVulkan {
				orderOK = false
			}
		}
	}
	return hasVulkan && hasSoftware && orderOK
}

// Auto mode must treat software-only as an explicit software choice, even
// when a Vulkan encode probe would otherwise pass.
func TestDetectSoftwareOnlyAutoHierarchyIsSoftwareOnly(t *testing.T) {
	runner := requireFFmpegRunner(t)
	allow := true

	caps, err := capabilities.Detect(context.Background(), runner, capabilities.DetectOptions{
		GPU:           softwareOnlyGPU(),
		VulkanEncoder: capabilities.VulkanEncoderAuto,
		AllowSoftware: &allow,
	})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if hierarchyHas(caps.EncoderHierarchy, capabilities.AccelVulkan) {
		t.Fatalf("Auto software-only EncoderHierarchy=%v want no vulkan", caps.EncoderHierarchy)
	}
	if len(caps.EncoderHierarchy) != 1 || caps.EncoderHierarchy[0] != capabilities.AccelNone {
		t.Fatalf("EncoderHierarchy=%v want [none]", caps.EncoderHierarchy)
	}
	if support, ok := caps.CodecMatrix[capabilities.CodecH264]; ok {
		if support.Preferred.Accel == capabilities.AccelVulkan || support.Preferred.Encoder == "h264_vulkan" {
			t.Fatalf("preferred %+v, Auto software-only must not select Vulkan", support.Preferred)
		}
	}
}

// Explicit fallback/on still includes Vulkan as the lowest-priority hardware
// backend when the selected device is software-only.
func TestDetectSoftwareOnlyFallbackIncludesVulkan(t *testing.T) {
	if os.Getenv("GOFFMPEG_SKIP_HW") == "1" {
		t.Skip("hardware tests disabled")
	}
	runner := requireFFmpegRunner(t)
	allow := true

	caps, err := capabilities.Detect(context.Background(), runner, capabilities.DetectOptions{
		GPU:           softwareOnlyGPU(),
		VulkanEncoder: capabilities.VulkanEncoderFallback,
		AllowSoftware: &allow,
	})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	enc, ok := caps.Encoders["h264_vulkan"]
	if !ok || !enc.Available {
		t.Skip("h264_vulkan unavailable on this host")
	}
	if !vulkanBeforeSoftware(caps.EncoderHierarchy) {
		t.Fatalf("EncoderHierarchy=%v want vulkan before software", caps.EncoderHierarchy)
	}
	support := caps.CodecMatrix[capabilities.CodecH264]
	if support.Preferred.Encoder != "h264_vulkan" || support.Preferred.Accel != capabilities.AccelVulkan {
		t.Fatalf("preferred %+v want h264_vulkan/vulkan", support.Preferred)
	}
}

package capabilities_test

import (
	"context"
	"os"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// Software-only device selection synthesizes AccelNone but must not be treated
// as an explicit hierarchy that blocks Vulkan after probes.
func TestDetectSoftwareOnlyIncludesVulkanInHierarchy(t *testing.T) {
	if os.Getenv("GOFFMPEG_SKIP_HW") == "1" {
		t.Skip("hardware tests disabled")
	}
	runner := requireFFmpegRunner(t)
	allow := true

	caps, err := capabilities.Detect(context.Background(), runner, capabilities.DetectOptions{
		GPU:           platform.GPUChoice{Mode: platform.GPUModeSoftware, SoftwareOnly: true, Enabled: true, Name: "CPU (software)"},
		VulkanEncoder: capabilities.VulkanEncoderAuto,
		AllowSoftware: &allow,
	})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	enc, ok := caps.Encoders["h264_vulkan"]
	if !ok || !enc.Available {
		t.Skip("h264_vulkan unavailable on this host")
	}
	var hasVulkan, hasSoftware bool
	vulkanBeforeSoftware := true
	for i, a := range caps.EncoderHierarchy {
		switch a {
		case capabilities.AccelVulkan:
			hasVulkan = true
			if i+1 < len(caps.EncoderHierarchy) && caps.EncoderHierarchy[i+1] != capabilities.AccelNone {
				vulkanBeforeSoftware = false
			}
		case capabilities.AccelNone:
			hasSoftware = true
			if i > 0 && caps.EncoderHierarchy[i-1] != capabilities.AccelVulkan {
				vulkanBeforeSoftware = false
			}
		}
	}
	if !hasVulkan || !hasSoftware || !vulkanBeforeSoftware {
		t.Fatalf("EncoderHierarchy=%v", caps.EncoderHierarchy)
	}
}

package capabilities

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// buildWarnCaps builds a capability matrix that reproduces the WSL NVENC +
// Vulkan failure shape: NVENC AV1 unsupported by the GPU, Vulkan probes failed
// with the same remediation across three codecs.
func buildWarnCaps() *Capabilities {
	caps := NewCapabilities()
	caps.VulkanEncoderRequested = true
	caps.DetectionHost = DetectionHost{
		Presence: platformGPUPresenceNVIDIA(),
		WSL:      true,
		DRI:      false,
	}
	vulkanHint := wslRemediation(true, false, true, "vulkan", HWProbeVulkanNoVideoEncode)
	caps.EncoderProbes = []EncoderProbeResult{
		{Encoder: "av1_nvenc", Accel: AccelNVENC, Code: HWProbeNVENCUnsupported, Hint: HintForProbeCode(HWProbeNVENCUnsupported)},
		{Encoder: "h264_vulkan", Accel: AccelVulkan, Code: HWProbeVulkanNoVideoEncode, Hint: vulkanHint},
		{Encoder: "hevc_vulkan", Accel: AccelVulkan, Code: HWProbeVulkanNoVideoEncode, Hint: vulkanHint},
		{Encoder: "av1_vulkan", Accel: AccelVulkan, Code: HWProbeVulkanNoVideoEncode, Hint: vulkanHint},
		{Encoder: "h264_nvenc", Accel: AccelNVENC, Available: true},
		{Encoder: "hevc_nvenc", Accel: AccelNVENC, Available: true},
	}
	return caps
}

func platformGPUPresenceNVIDIA() platform.GPUPresence {
	return platform.GPUPresence{NVIDIA: true, WSLDXG: true}
}

// The merged emitter must produce exactly one hardware line and one Vulkan
// line, no matter how many encoders shared the same remediation.
func TestStartupHardwareWarnLinesDeduplicated(t *testing.T) {
	caps := buildWarnCaps()
	lines := caps.StartupHardwareWarnLines()

	hardwareCount, vulkanCount := 0, 0
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "hardware probe:"):
			hardwareCount++
		case strings.HasPrefix(line, "vulkan probe:"):
			vulkanCount++
			for _, enc := range []string{"av1_vulkan", "h264_vulkan", "hevc_vulkan"} {
				if !strings.Contains(line, enc) {
					t.Fatalf("vulkan line %q should name %s", line, enc)
				}
			}
		}
	}
	if hardwareCount != 0 {
		t.Fatalf("hardware warn lines = %d, want 0 (capability limits are info): %v", hardwareCount, lines)
	}
	if vulkanCount != 1 {
		t.Fatalf("vulkan warn lines = %d, want 1: %v", vulkanCount, lines)
	}
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1: %v", len(lines), lines)
	}
	info := caps.StartupHardwareInfoLines()
	if len(info) != 1 || !strings.Contains(info[0], "av1_nvenc") {
		t.Fatalf("info lines = %v, want one line naming av1_nvenc", info)
	}
}

// The two legacy emitters must not both report the same remediation text.
func TestLegacyEmittersDoNotDuplicate(t *testing.T) {
	caps := buildWarnCaps()
	hw := caps.GPUPresentRemediationLogLines()
	vulkan := caps.VulkanProbeWarnLines()

	for _, a := range hw {
		for _, b := range vulkan {
			if strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) {
				t.Fatalf("duplicate line across emitters: %q", a)
			}
		}
	}
	// The capability-limit line is not a gpu_present row, so the legacy
	// gpu_present view (used by older callers) must stay empty here.
	if len(hw) != 0 {
		t.Fatalf("gpu_present rows = %v, want none (only a capability limit was present)", hw)
	}
	if len(vulkan) != 1 {
		t.Fatalf("vulkan lines = %v, want 1", vulkan)
	}
}

func TestStartupHardwareWarnLinesNoVulkanWhenNotRequested(t *testing.T) {
	caps := buildWarnCaps()
	caps.VulkanEncoderRequested = false
	for _, line := range caps.StartupHardwareWarnLines() {
		if strings.HasPrefix(line, "vulkan probe:") {
			t.Fatalf("unexpected vulkan line when Vulkan was not requested: %q", line)
		}
	}
}

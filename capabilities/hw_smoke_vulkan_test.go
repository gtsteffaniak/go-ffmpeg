package capabilities

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestHwSmokeArgsVulkanLinux(t *testing.T) {
	plat := platform.Info{
		OS:      "linux",
		Vulkan:  true,
		Details: map[string]string{"render_device": "/dev/dri/renderD128"},
	}
	args := hwSmokeArgs("h264_vulkan", plat)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "drm=dr:/dev/dri/renderD128") || !strings.Contains(joined, "vulkan=vk@dr") {
		t.Fatalf("missing vulkan device init: %v", args)
	}
	if !strings.Contains(joined, "h264_vulkan") || !strings.Contains(joined, "hwupload") {
		t.Fatalf("unexpected smoke args: %v", args)
	}
}

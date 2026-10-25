package platform_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestFFmpegVulkanDeviceInitLinuxRenderNode(t *testing.T) {
	plat := platform.Info{OS: "linux", Details: map[string]string{"render_device": "/dev/dri/renderD128"}}
	args := platform.FFmpegVulkanDeviceInit(platform.GPUChoice{}, plat)
	want := []string{
		"-init_hw_device", "drm=dr:/dev/dri/renderD128",
		"-init_hw_device", "vulkan=vk@dr",
		"-filter_hw_device", "vk",
	}
	if len(args) != len(want) {
		t.Fatalf("args=%v want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d]=%q want %q", i, args[i], want[i])
		}
	}
}

func TestFFmpegVulkanDeviceInitWindows(t *testing.T) {
	plat := platform.Info{OS: "windows"}
	args := platform.FFmpegVulkanDeviceInit(platform.GPUChoice{}, plat)
	if len(args) < 2 || args[1] != "vulkan=vk" {
		t.Fatalf("args=%v", args)
	}
}

package capabilities

import (
	"os"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestVulkanPrereqLoaderMissing(t *testing.T) {
	deps := prereqDeps{
		Glob:     func(string) ([]string, error) { return nil, nil },
		Open:     func(string) (*os.File, error) { return nil, os.ErrNotExist },
		Stat:     func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFile: func(string) ([]byte, error) { return nil, os.ErrNotExist },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Ldconfig: func() ([]byte, error) { return nil, os.ErrNotExist },
		Env:      func(string) string { return "" },
	}
	plat := platform.Info{OS: "linux", Vulkan: true, DRI: true}
	code, _ := checkVulkanPrereq(plat, deps)
	if code != HWProbeVulkanLoaderMissing {
		t.Fatalf("code=%s", code)
	}
}

func TestClassifyVulkanVideoEncode(t *testing.T) {
	code, hint := ClassifyHWProbeFailure("VK_KHR_video_encode_queue not supported", nil, "vulkan")
	if code != HWProbeVulkanNoVideoEncode || hint == "" {
		t.Fatalf("code=%s hint=%q", code, hint)
	}
}

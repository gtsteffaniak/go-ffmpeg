package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Vulkan reports whether the host can load Vulkan and access a GPU device for FFmpeg.
// macOS is excluded (FFmpeg Vulkan video encode is not supported there).
func vulkanStackAvailable() bool {
	return vulkanStackAvailableWithDeps(defaultVulkanDeps())
}

type vulkanDeps struct {
	LookPath func(file string) (string, error)
	Stat     func(name string) (os.FileInfo, error)
	Glob     func(pattern string) ([]string, error)
	Open     func(name string) (*os.File, error)
}

func defaultVulkanDeps() vulkanDeps {
	return vulkanDeps{
		LookPath: exec.LookPath,
		Stat:     os.Stat,
		Glob:     filepath.Glob,
		Open:     os.Open,
	}
}

func vulkanStackAvailableWithDeps(deps vulkanDeps) bool {
	if runtime.GOOS == "darwin" {
		return false
	}
	if pi := DetectPiModel(); pi == PiLegacy {
		return false
	}
	if !libVulkanResolvable(deps) {
		return false
	}
	switch runtime.GOOS {
	case "windows":
		return true
	default:
		return vulkanRenderNodeOpenable(deps)
	}
}

func libVulkanResolvable(deps vulkanDeps) bool {
	if runtime.GOOS == "windows" {
		for _, p := range []string{
			"C:\\Windows\\System32\\vulkan-1.dll",
			"C:\\Windows\\SysWOW64\\vulkan-1.dll",
		} {
			if _, err := deps.Stat(p); err == nil {
				return true
			}
		}
		if path, err := deps.LookPath("vulkan-1.dll"); err == nil && path != "" {
			return true
		}
		return false
	}
	if path, err := deps.LookPath("libvulkan.so.1"); err == nil && path != "" {
		return true
	}
	if matches, _ := deps.Glob("/usr/lib/*-linux-gnu/libvulkan.so.1"); len(matches) > 0 {
		return true
	}
	for _, p := range []string{
		"/usr/lib/x86_64-linux-gnu/libvulkan.so.1",
		"/usr/lib/aarch64-linux-gnu/libvulkan.so.1",
		"/usr/lib64/libvulkan.so.1",
		"/usr/lib/libvulkan.so.1",
		"/usr/lib/wsl/lib/libvulkan.so.1",
	} {
		if _, err := deps.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func vulkanRenderNodeOpenable(deps vulkanDeps) bool {
	nodes, _ := deps.Glob("/dev/dri/renderD*")
	for _, node := range nodes {
		f, err := deps.Open(node)
		if err != nil {
			continue
		}
		f.Close()
		return true
	}
	for _, node := range []string{"/dev/dxg"} {
		if _, err := deps.Stat(node); err == nil {
			return true
		}
	}
	return false
}

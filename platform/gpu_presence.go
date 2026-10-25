package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// GPUPresence records PCI GPUs seen on the host independent of working drivers.
type GPUPresence struct {
	Intel  bool `json:"intel,omitempty"`
	AMD    bool `json:"amd,omitempty"`
	NVIDIA bool `json:"nvidia,omitempty"`
	WSLDXG bool `json:"wslDxg,omitempty"` // /dev/dxg (WSL GPU paravirt)
}

// Any reports whether any GPU class was detected.
func (p GPUPresence) Any() bool {
	return p.Intel || p.AMD || p.NVIDIA || p.WSLDXG
}

type gpuPresenceDeps struct {
	Glob     func(pattern string) ([]string, error)
	ReadFile func(name string) ([]byte, error)
	Stat     func(name string) (os.FileInfo, error)
}

func defaultGPUPresenceDeps() gpuPresenceDeps {
	return gpuPresenceDeps{
		Glob:     filepath.Glob,
		ReadFile: os.ReadFile,
		Stat:     os.Stat,
	}
}

// DetectGPUPresence probes sysfs DRM vendors and common device nodes without requiring VA/NVENC drivers.
func DetectGPUPresence() GPUPresence {
	return detectGPUPresence(defaultGPUPresenceDeps())
}

func detectGPUPresence(deps gpuPresenceDeps) GPUPresence {
	var p GPUPresence
	cardVendors, _ := deps.Glob("/sys/class/drm/card*/device/vendor")
	for _, path := range cardVendors {
		data, err := deps.ReadFile(path)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(string(data)) {
		case "0x8086":
			p.Intel = true
		case "0x1002":
			p.AMD = true
		case "0x10de":
			p.NVIDIA = true
		}
	}
	for _, node := range []string{"/dev/nvidia0", "/dev/nvidiactl", "/dev/nvidia-uvm"} {
		if _, err := deps.Stat(node); err == nil {
			p.NVIDIA = true
		}
	}
	if _, err := deps.Stat("/dev/dxg"); err == nil {
		p.WSLDXG = true
	}
	return p
}

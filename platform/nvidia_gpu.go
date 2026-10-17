package platform

import (
	"strings"
)

// NVIDIAGPUName returns a human-readable NVIDIA GPU name from nvidia-smi when available.
func NVIDIAGPUName() string {
	out, err := combinedOutputTimeout("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return ""
	}
	// First GPU when multiple are listed.
	if idx := strings.Index(line, "\n"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	return line
}

func hasNVIDIAGPUDevice(gpus []GPUDevice) bool {
	for _, g := range gpus {
		if g.Vendor == "nvidia" {
			return true
		}
	}
	return false
}

func hasDiscreteDRMDevice(gpus []GPUDevice) bool {
	for _, g := range gpus {
		if !g.Integrated {
			return true
		}
	}
	return false
}

func syntheticNVIDIAGPU() GPUDevice {
	name := NVIDIAGPUName()
	if name == "" {
		name = "NVIDIA GPU"
	}
	return GPUDevice{
		Vendor:     "nvidia",
		Name:       name,
		Integrated: false,
	}
}

func wslNVIDIAGPUChoice(label string) GPUChoice {
	g := syntheticNVIDIAGPU()
	return GPUChoice{
		Enabled: true,
		Mode:    GPUModeDGPU,
		Label:   label,
		Vendor:  g.Vendor,
		Name:    g.Name,
	}
}

func shouldPreferWSLNVIDIAAuto(env GPUResolveEnv) bool {
	return env.GOOS == "linux" && env.WSL && env.NVIDIA && !hasNVIDIAGPUDevice(env.Devices)
}

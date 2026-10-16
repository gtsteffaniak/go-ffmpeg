package capabilities

import (
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// AccelGPU describes which GPU a hardware backend is expected to use in reports.
type AccelGPU struct {
	Name       string
	RenderNode string
	Vendor     string
}

// GPUForAccel resolves the likely GPU device for a backend when no GPU is explicitly selected.
func GPUForAccel(c *Capabilities, accel AccelType) AccelGPU {
	if c == nil {
		return AccelGPU{}
	}
	if c.SelectedGPU != nil && c.SelectedGPU.Enabled {
		dev := c.Platform.Details["render_device"]
		if dev == "" {
			dev = c.SelectedGPU.RenderDevice
		}
		return AccelGPU{
			Name:       c.SelectedGPU.Name,
			RenderNode: dev,
			Vendor:     c.SelectedGPU.Vendor,
		}
	}
	if accel == AccelNone || accel == AccelVideoToolbox {
		if accel == AccelVideoToolbox {
			return AccelGPU{Name: "Apple GPU", Vendor: "apple"}
		}
		return AccelGPU{Name: "CPU", Vendor: "software"}
	}

	gpus, err := platform.ListGPUs()
	if err != nil || len(gpus) == 0 {
		return AccelGPU{Name: vendorLabelForAccel(accel, c.Platform)}
	}

	switch accel {
	case AccelNVENC:
		if g := pickGPU(gpus, "nvidia", false); g != nil {
			return drmToAccelGPU(*g)
		}
	case AccelAMF:
		if g := pickGPU(gpus, "amd", false); g != nil {
			return drmToAccelGPU(*g)
		}
		if g := pickGPU(gpus, "amd", true); g != nil {
			return drmToAccelGPU(*g)
		}
	case AccelQSV, AccelVAAPI, AccelD3D12:
		if g := pickGPU(gpus, "intel", true); g != nil {
			return drmToAccelGPU(*g)
		}
		if g := pickGPU(gpus, "amd", true); g != nil {
			return drmToAccelGPU(*g)
		}
		if g := pickGPU(gpus, "amd", false); g != nil {
			return drmToAccelGPU(*g)
		}
	}
	return AccelGPU{Name: vendorLabelForAccel(accel, c.Platform)}
}

func drmToAccelGPU(g platform.GPUDevice) AccelGPU {
	return AccelGPU{
		Name:       g.Name,
		RenderNode: g.RenderNode,
		Vendor:     g.Vendor,
	}
}

func pickGPU(gpus []platform.GPUDevice, vendor string, integrated bool) *platform.GPUDevice {
	var fallback *platform.GPUDevice
	for _, g := range gpus {
		if g.Vendor != vendor {
			continue
		}
		if g.Integrated == integrated {
			return &g
		}
		if fallback == nil {
			fallback = &g
		}
	}
	return fallback
}

func vendorLabelForAccel(accel AccelType, plat PlatformInfo) string {
	switch accel {
	case AccelNVENC:
		if plat.NVIDIA {
			return "NVIDIA GPU"
		}
	case AccelAMF:
		if plat.AMD {
			return "AMD GPU"
		}
	case AccelQSV, AccelVAAPI:
		if plat.Intel {
			return "Intel GPU"
		}
		if plat.AMD {
			return "AMD GPU"
		}
	}
	return AccelLabel(accel)
}

// FormatAccelGPUPath returns a human-readable GPU suffix for codec resolution lines.
func FormatAccelGPUPath(c *Capabilities, accel AccelType) string {
	if accel == AccelNone {
		return ""
	}
	gpu := GPUForAccel(c, accel)
	parts := []string{AccelLabel(accel)}
	if gpu.Name != "" && !strings.EqualFold(gpu.Name, "CPU") {
		parts = append(parts, gpu.Name)
	}
	if gpu.RenderNode != "" {
		parts = append(parts, gpu.RenderNode)
	}
	return " (" + strings.Join(parts, " · ") + ")"
}

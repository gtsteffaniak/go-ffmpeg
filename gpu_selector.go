package ffmpeg

import (
	"fmt"
	"strings"
)

// GPUKind selects which GPU to use for hardware acceleration.
type GPUKind int

const (
	// GPUAuto (zero) uses go-ffmpeg's automatic device hierarchy: integrated/SoC GPUs first,
	// then discrete GPUs, then software, with per-vendor backend order on each device and
	// per-job fallback along that chain. Use DevicePreference PreferDiscrete to try dGPU first.
	GPUAuto GPUKind = 0
	// GPUIntegrated scopes to the integrated GPU (Intel/AMD iGPU).
	GPUIntegrated GPUKind = 1
	// GPUDiscrete scopes to the discrete GPU (e.g. NVIDIA dGPU).
	GPUDiscrete GPUKind = 2
	// GPUSoftware forces CPU-only encode/decode (no hardware smoke tests).
	GPUSoftware GPUKind = 3
	// GPUDevice selects by RenderNode or NameMatch on the selector.
	GPUDevice GPUKind = 4
)

// GPUSelector is the typed GPU configuration. Zero value is GPUAuto.
type GPUSelector struct {
	Kind       GPUKind
	RenderNode string // Linux DRM path when Kind == GPUDevice
	NameMatch  string // substring match when Kind == GPUDevice
}

// GPUStrictness controls behavior when the requested GPU is unavailable.
type GPUStrictness int

const (
	// GPUFallbackOnUnavailable (zero) falls back to auto with a reported reason.
	GPUFallbackOnUnavailable GPUStrictness = 0
	// GPUStrictOnUnavailable returns an error from ResolveGPUSelector.
	GPUStrictOnUnavailable GPUStrictness = 1
)

// GPUByRenderNode selects a Linux DRM render node (e.g. /dev/dri/renderD129).
func GPUByRenderNode(path string) GPUSelector {
	return GPUSelector{Kind: GPUDevice, RenderNode: strings.TrimSpace(path)}
}

// GPUByName selects a GPU whose name or PCI description contains substr.
func GPUByName(substr string) GPUSelector {
	return GPUSelector{Kind: GPUDevice, NameMatch: strings.TrimSpace(substr)}
}

// String returns a stable label for logs and capability reports.
func (s GPUSelector) String() string {
	switch s.Kind {
	case GPUAuto:
		return "auto"
	case GPUIntegrated:
		return "igpu"
	case GPUDiscrete:
		return "dgpu"
	case GPUSoftware:
		return "software"
	case GPUDevice:
		if s.RenderNode != "" {
			return s.RenderNode
		}
		if s.NameMatch != "" {
			return "name:" + s.NameMatch
		}
		return "device"
	default:
		return fmt.Sprintf("gpu(%d)", s.Kind)
	}
}

// IsZero reports whether the selector is unset (equivalent to auto).
func (s GPUSelector) IsZero() bool {
	return s.Kind == GPUAuto && s.RenderNode == "" && s.NameMatch == ""
}

func (c *Config) effectiveGPUSelector() GPUSelector {
	if !c.GPUSelector.IsZero() {
		return c.GPUSelector
	}
	if strings.TrimSpace(c.GPU) == "" {
		return GPUSelector{Kind: GPUAuto}
	}
	sel, err := ParseGPUSelector(c.GPU)
	if err != nil {
		return GPUSelector{Kind: GPUAuto}
	}
	return sel
}

func (c *Config) effectiveGPUStrictness() GPUStrictness {
	return c.GPUStrictness
}

func (c *Config) effectiveDeviceSelection() DeviceSelection {
	ds := c.DeviceSelection
	if ds.Primary.IsZero() {
		ds.Primary = c.effectiveGPUSelector()
	}
	return ds
}

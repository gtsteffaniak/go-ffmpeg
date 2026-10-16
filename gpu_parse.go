package ffmpeg

import (
	"strings"
)

// ParseGPUSelector maps FileBrowser-style YAML strings to a typed GPUSelector.
// It does not touch the host or validate render nodes; use ResolveGPUSelector for that.
func ParseGPUSelector(spec string) (GPUSelector, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return GPUSelector{Kind: GPUAuto}, nil
	}
	key := strings.ToLower(spec)
	switch key {
	case "auto":
		return GPUSelector{Kind: GPUAuto}, nil
	case "igpu", "integrated":
		return GPUSelector{Kind: GPUIntegrated}, nil
	case "dgpu", "discrete":
		return GPUSelector{Kind: GPUDiscrete}, nil
	case "software", "cpu", "none":
		return GPUSelector{Kind: GPUSoftware}, nil
	}
	if strings.HasPrefix(spec, "/dev/") {
		return GPUByRenderNode(spec), nil
	}
	return GPUByName(spec), nil
}

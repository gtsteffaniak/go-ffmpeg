package platform

import (
	"fmt"
	"os"
)

// SelectionKind identifies a typed GPU selection request from the ffmpeg package.
type SelectionKind int

const (
	SelectionAuto         SelectionKind = 0
	SelectionIntegrated   SelectionKind = 1
	SelectionDiscrete     SelectionKind = 2
	SelectionSoftware     SelectionKind = 3
	SelectionDevice       SelectionKind = 4
)

// GPURequest is the platform-neutral GPU selection input.
type GPURequest struct {
	Kind       SelectionKind
	RenderNode string
	NameMatch  string
	Label      string // selector summary for reports
}

// ResolveGPURequest resolves a typed GPU request against the host environment.
func ResolveGPURequest(req GPURequest, env GPUResolveEnv, strict bool) (GPUChoice, error) {
	switch req.Kind {
	case SelectionAuto:
		return GPUChoice{Mode: GPUModeAuto, AutoSelect: true, Label: req.Label}, nil
	case SelectionSoftware:
		return GPUChoice{
			Mode:         GPUModeSoftware,
			SoftwareOnly: true,
			Name:         "CPU (software)",
			Label:        req.Label,
		}, nil
	case SelectionIntegrated:
		return resolveIntegrated(req, env, strict)
	case SelectionDiscrete:
		return resolveDiscrete(req, env, strict)
	case SelectionDevice:
		if req.RenderNode != "" {
			return resolveRenderNode(req, env, strict)
		}
		return resolveNameDevice(req, env, strict)
	default:
		if strict {
			return GPUChoice{}, fmt.Errorf("invalid gpu selection kind %d", req.Kind)
		}
		return fallbackAuto(req.Label, "invalid gpu selection; falling back to auto")
	}
}

// ResolveGPURequestDefaultEnv resolves using live host detection.
func ResolveGPURequestDefaultEnv(req GPURequest, strict bool) (GPUChoice, error) {
	return ResolveGPURequest(req, defaultGPUResolveEnv(), strict)
}

func resolveIntegrated(req GPURequest, env GPUResolveEnv, strict bool) (GPUChoice, error) {
	switch env.GOOS {
	case "darwin":
		return GPUChoice{
			Enabled: true,
			Mode:    GPUModeIGPU,
			Label:   req.Label,
			Vendor:  "apple",
			Name:    "Apple GPU (VideoToolbox)",
		}, nil
	case "windows":
		if env.Intel {
			return GPUChoice{Enabled: true, Mode: GPUModeIGPU, Label: req.Label, Vendor: "intel", Name: "Intel GPU"}, nil
		}
		if env.AMD {
			return GPUChoice{Enabled: true, Mode: GPUModeIGPU, Label: req.Label, Vendor: "amd", Name: "AMD integrated GPU"}, nil
		}
		return unavailable(strict, req.Label, "no integrated GPU detected")
	}
	for _, d := range env.Devices {
		if d.Integrated {
			c := gpuChoiceFromDRM(d)
			c.Mode = GPUModeIGPU
			c.Label = req.Label
			return c, nil
		}
	}
	return unavailable(strict, req.Label, "no integrated GPU render node found")
}

func resolveDiscrete(req GPURequest, env GPUResolveEnv, strict bool) (GPUChoice, error) {
	switch env.GOOS {
	case "darwin":
		return unavailable(strict, req.Label, "discrete GPU selection is not applicable on macOS")
	case "windows":
		if env.NVIDIA {
			return GPUChoice{Enabled: true, Mode: GPUModeDGPU, Label: req.Label, Vendor: "nvidia", Name: "NVIDIA GPU"}, nil
		}
		if env.AMD {
			return GPUChoice{Enabled: true, Mode: GPUModeDGPU, Label: req.Label, Vendor: "amd", Name: "AMD discrete GPU"}, nil
		}
		return unavailable(strict, req.Label, "no discrete GPU detected")
	}
	for _, d := range env.Devices {
		if !d.Integrated {
			c := gpuChoiceFromDRM(d)
			c.Mode = GPUModeDGPU
			c.Label = req.Label
			return c, nil
		}
	}
	return unavailable(strict, req.Label, "no discrete GPU render node found")
}

func resolveRenderNode(req GPURequest, env GPUResolveEnv, strict bool) (GPUChoice, error) {
	path := req.RenderNode
	stat := env.Stat
	if stat == nil {
		stat = func(p string) error { _, err := os.Stat(p); return err }
	}
	if err := stat(path); err != nil {
		return GPUChoice{}, fmt.Errorf("gpu render node %q not accessible: %w", path, err)
	}
	for _, d := range env.Devices {
		if d.RenderNode == path {
			c := gpuChoiceFromDRM(d)
			c.Mode = GPUModeDevice
			c.Label = req.Label
			return c, nil
		}
	}
	return GPUChoice{
		Enabled:      true,
		Mode:         GPUModeDevice,
		Label:        req.Label,
		RenderDevice: path,
		Vendor:       vendorFromRenderNode(path),
		Name:         path,
	}, nil
}

func resolveNameDevice(req GPURequest, env GPUResolveEnv, strict bool) (GPUChoice, error) {
	spec := req.NameMatch
	if c, ok, err := matchGPUName(spec, env); err != nil {
		return GPUChoice{}, err
	} else if ok {
		c.Mode = GPUModeDevice
		c.Label = req.Label
		return c, nil
	}
	return unavailable(strict, req.Label, fmt.Sprintf("no GPU matching %q", spec))
}

func unavailable(strict bool, label, msg string) (GPUChoice, error) {
	if strict {
		return GPUChoice{}, fmt.Errorf("%s (selector %q)", msg, label)
	}
	return fallbackAuto(label, msg)
}

func fallbackAuto(label, msg string) (GPUChoice, error) {
	return GPUChoice{
		Mode:           GPUModeAuto,
		AutoSelect:     true,
		Label:          label,
		Fallback:       true,
		FallbackReason: msg,
		Warning:        msg,
	}, nil
}

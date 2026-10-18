package ffmpeg

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// DeviceSelection configures explicit primary GPU and ordered fallbacks per job.
type DeviceSelection struct {
	Primary   GPUSelector
	Fallbacks []GPUSelector
	// AllowSoftware overrides Config.AllowSoftwareEncoderFallback when non-nil.
	AllowSoftware *bool
}

// ParseGPUSelectorList parses a comma-separated ordered GPU selector list (primary first).
func ParseGPUSelectorList(spec string) ([]GPUSelector, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	parts := strings.Split(spec, ",")
	var out []GPUSelector
	for _, p := range parts {
		sel, err := ParseGPUSelector(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		out = append(out, sel)
	}
	return out, nil
}

// ResolveDeviceSelection resolves primary + fallbacks to concrete GPU choices.
func ResolveDeviceSelection(sel DeviceSelection, strict GPUStrictness) (primary platform.GPUChoice, fallbacks []platform.GPUChoice, err error) {
	return ResolveDeviceSelectionWithEnv(sel, strict, platform.DefaultGPUResolveEnv())
}

// ResolveDeviceSelectionWithEnv resolves using an injected environment (tests).
func ResolveDeviceSelectionWithEnv(sel DeviceSelection, strict GPUStrictness, env platform.GPUResolveEnv) (primary platform.GPUChoice, fallbacks []platform.GPUChoice, err error) {
	choices, err := resolveSelectorChainEnv(sel.Primary, sel.Fallbacks, strict, env)
	if err != nil {
		return platform.GPUChoice{}, nil, err
	}
	if len(choices) == 0 {
		return platform.GPUChoice{Mode: platform.GPUModeAuto, AutoSelect: true}, nil, nil
	}
	primary = choices[0]
	if len(choices) > 1 {
		fallbacks = choices[1:]
	}
	return primary, fallbacks, nil
}

func resolveSelectorChain(primary GPUSelector, fallbacks []GPUSelector, strict GPUStrictness) ([]platform.GPUChoice, error) {
	return resolveSelectorChainEnv(primary, fallbacks, strict, platform.DefaultGPUResolveEnv())
}

func resolveSelectorChainEnv(primary GPUSelector, fallbacks []GPUSelector, strict GPUStrictness, env platform.GPUResolveEnv) ([]platform.GPUChoice, error) {
	var specs []GPUSelector
	if !primary.IsZero() || primary.Kind == GPUSoftware {
		specs = append(specs, primary)
	}
	specs = append(specs, fallbacks...)
	if len(specs) == 0 {
		return nil, nil
	}
	devices := env.Devices
	if len(devices) == 0 {
		devices, _ = platform.ListGPUDevices()
	}
	var out []platform.GPUChoice
	for i, spec := range specs {
		c, err := ResolveGPUSelectorWithEnv(spec, strict, env)
		if err != nil {
			return nil, err
		}
		platform.FillGPUChoiceDevice(&c, devices)
		if c.Fallback && strict == GPUStrictOnUnavailable {
			return nil, fmt.Errorf("gpu %q unavailable (strict)", spec.String())
		}
		if c.Fallback && i == 0 && strict == GPUStrictOnUnavailable {
			return nil, fmt.Errorf("primary gpu %q unavailable: %s", spec.String(), c.FallbackReason)
		}
		out = append(out, c)
	}
	return out, nil
}

// ListDevices returns stable GPU entries for admin dropdowns.
func ListDevices() ([]platform.GPUDevice, error) {
	return platform.ListGPUDevices()
}

// DetectDevices is an alias for ListDevices.
func DetectDevices() ([]platform.GPUDevice, error) {
	return ListDevices()
}

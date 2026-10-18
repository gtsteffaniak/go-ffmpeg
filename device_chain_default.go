package ffmpeg

import (
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func effectiveAllowSoftware(cfg Config, ds DeviceSelection) bool {
	allow := true
	if cfg.AllowSoftwareEncoderFallback != nil {
		allow = *cfg.AllowSoftwareEncoderFallback
	}
	if ds.AllowSoftware != nil {
		allow = *ds.AllowSoftware
	}
	return allow
}

func deviceSelectionExplicit(ds DeviceSelection) bool {
	if len(ds.Fallbacks) > 0 {
		return true
	}
	if ds.Primary.Kind == GPUSoftware {
		return true
	}
	return !ds.Primary.IsZero() && ds.Primary.Kind != GPUAuto
}

// ResolveConfigDeviceChain builds the ordered GPU chain for detection and encode.
// Precedence: explicit DeviceSelection → legacy non-auto GPUSelector → default order from DevicePreference.
func ResolveConfigDeviceChain(cfg Config, strict GPUStrictness, env platform.GPUResolveEnv) (chain []platform.GPUChoice, orderSummary string, err error) {
	if env.GOOS == "" {
		env = platform.DefaultGPUResolveEnv()
	}
	ds := cfg.DeviceSelection
	allow := effectiveAllowSoftware(cfg, ds)

	if deviceSelectionExplicit(ds) {
		choices, err := resolveSelectorChainEnv(ds.Primary, ds.Fallbacks, strict, env)
		if err != nil {
			return nil, "", err
		}
		if allow {
			choices = append(choices, softwareDeviceChoice())
		}
		return choices, formatDeviceOrderLabel(choices, cfg.effectiveDevicePreference(), true), nil
	}

	sel := cfg.effectiveGPUSelector()
	if !sel.IsZero() {
		c, err := ResolveGPUSelectorWithEnv(sel, strict, env)
		if err != nil {
			return nil, "", err
		}
		platform.FillGPUChoiceDevice(&c, env.Devices)
		chain := []platform.GPUChoice{c}
		if allow && !c.SoftwareOnly {
			chain = append(chain, softwareDeviceChoice())
		}
		return chain, formatDeviceOrderLabel(chain, cfg.effectiveDevicePreference(), true), nil
	}

	pref := cfg.effectiveDevicePreference()
	chain, err = buildDefaultDeviceChain(env, pref, allow)
	if err != nil {
		return nil, "", err
	}
	label := formatDeviceOrderLabel(chain, pref, false)
	return chain, label, nil
}

func buildDefaultDeviceChain(env platform.GPUResolveEnv, pref DevicePreference, allowSoftware bool) ([]platform.GPUChoice, error) {
	devices := env.Devices
	if len(devices) == 0 {
		list, err := platform.ListGPUDevices()
		if err == nil {
			devices = list
		}
	}
	devices = appendPiSOCDevice(devices)
	ordered := orderGPUDevices(devices, pref)
	var chain []platform.GPUChoice
	for _, d := range ordered {
		c := platform.GPUChoiceFromDevice(d)
		platform.FillGPUChoiceDevice(&c, devices)
		chain = append(chain, c)
	}
	if len(chain) == 0 {
		chain = append(chain, platform.GPUChoice{Mode: platform.GPUModeAuto, AutoSelect: true, Label: "auto"})
	}
	if allowSoftware {
		chain = append(chain, softwareDeviceChoice())
	}
	return chain, nil
}

func appendPiSOCDevice(devices []platform.GPUDevice) []platform.GPUDevice {
	model := platform.DetectPiModel()
	if model == platform.PiUnknown {
		return devices
	}
	for _, d := range devices {
		if d.DeviceClass == "soc" {
			return devices
		}
	}
	name := "Raspberry Pi SoC"
	if model == platform.Pi5 {
		name = "Raspberry Pi 5 SoC"
	} else if model == platform.Pi4 {
		name = "Raspberry Pi 4 SoC"
	}
	return append(devices, platform.GPUDevice{
		ID:          "soc:" + string(model),
		Name:        name,
		Vendor:      "soc",
		DeviceClass: "soc",
		Integrated:  true,
	})
}

func orderGPUDevices(devices []platform.GPUDevice, pref DevicePreference) []platform.GPUDevice {
	var soc, igpu, dgpu []platform.GPUDevice
	for _, d := range devices {
		switch {
		case d.DeviceClass == "soc":
			soc = append(soc, d)
		case d.Integrated || d.Vendor == "intel" && isIntegratedClass(d):
			igpu = append(igpu, d)
		default:
			dgpu = append(dgpu, d)
		}
	}
	integrated := append(soc, igpu...)
	var hw []platform.GPUDevice
	if pref == PreferDiscrete {
		hw = append(hw, dgpu...)
		hw = append(hw, integrated...)
	} else {
		hw = append(hw, integrated...)
		hw = append(hw, dgpu...)
	}
	return hw
}

func isIntegratedClass(d platform.GPUDevice) bool {
	return d.Integrated || d.DeviceClass == "igpu"
}

func softwareDeviceChoice() platform.GPUChoice {
	return platform.GPUChoice{
		Mode:         platform.GPUModeSoftware,
		SoftwareOnly: true,
		Name:         "CPU (software)",
		Label:        "software",
	}
}

func formatDeviceOrderLabel(chain []platform.GPUChoice, pref DevicePreference, explicit bool) string {
	var parts []string
	for _, c := range chain {
		if c.SoftwareOnly {
			parts = append(parts, "software")
			continue
		}
		if c.Name != "" {
			parts = append(parts, c.Name)
		} else if c.Label != "" {
			parts = append(parts, c.Label)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	prefix := "Device order (auto)"
	if explicit {
		prefix = "Device order (explicit)"
	} else if pref == PreferDiscrete {
		prefix = "Device order (prefer discrete)"
	}
	return prefix + ": " + strings.Join(parts, " → ")
}

// DefaultGPUResolveEnv is exported for tests.
func DefaultGPUResolveEnv() platform.GPUResolveEnv {
	return platform.DefaultGPUResolveEnv()
}

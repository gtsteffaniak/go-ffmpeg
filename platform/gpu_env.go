package platform

import (
	"os"
	"runtime"
	"strings"
)

// GPUResolveEnv supplies host facts for GPU resolution. Tests inject synthetic envs.
type GPUResolveEnv struct {
	GOOS          string
	WSL           bool
	Devices       []GPUDevice
	NVIDIA        bool
	Intel         bool
	AMD           bool
	PCIGPUStrings []string
	Stat          func(path string) error
}

func defaultGPUResolveEnv() GPUResolveEnv {
	env := GPUResolveEnv{
		GOOS:   runtime.GOOS,
		WSL:    isWSLEnvironment(),
		Stat: func(p string) error { _, err := os.Stat(p); return err },
		NVIDIA: nvidiaAvailable(),
		Intel:  intelGPUPresent(),
		AMD:    amdGPUPresent(),
	}
	if runtime.GOOS == "linux" {
		devs, err := listDRMGPUs()
		if err == nil {
			env.Devices = devs
		}
	}
	if desc := detectGPUDescription(); desc != "" {
		for _, part := range strings.Split(desc, ";") {
			part = strings.TrimSpace(part)
			if part != "" {
				env.PCIGPUStrings = append(env.PCIGPUStrings, part)
			}
		}
	}
	return env
}

func matchGPUName(spec string, env GPUResolveEnv) (GPUChoice, bool, error) {
	for _, d := range env.Devices {
		if nameMatches(d.Name, spec) || nameMatches(d.RenderNode, spec) {
			c := gpuChoiceFromDRM(d)
			return c, true, nil
		}
	}
	for _, part := range env.PCIGPUStrings {
		if nameMatches(part, spec) {
			vendor := vendorFromDescription(part)
			for _, d := range env.Devices {
				if d.Vendor == vendor || nameMatches(d.Name, part) {
					c := gpuChoiceFromDRM(d)
					return c, true, nil
				}
			}
			return GPUChoice{Enabled: true, Vendor: vendor, Name: part}, true, nil
		}
	}
	if env.NVIDIA && nameMatches("nvidia", spec) {
		return GPUChoice{Enabled: true, Vendor: "nvidia", Name: spec}, true, nil
	}
	return GPUChoice{}, false, nil
}

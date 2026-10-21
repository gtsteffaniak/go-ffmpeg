package platform

import (
	"strings"
)

// listDarwinGPUs enumerates GPUs on macOS via system_profiler. VideoToolbox
// has no DRM render node or WMI entry, so enumeration comes from the
// Graphics/Displays report — "Chipset Model" names every GPU present,
// integrated (Apple Silicon, Intel graphics) or discrete (AMD, eGPU).
func listDarwinGPUs() []GPUDevice {
	out, err := combinedOutputTimeout("system_profiler", "SPDisplaysDataType")
	if err != nil {
		return nil
	}
	var gpus []GPUDevice
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Chipset Model:") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(line, "Chipset Model:"))
		if name == "" {
			continue
		}
		vendor := darwinGPUVendor(name)
		gpus = append(gpus, GPUDevice{
			Vendor:     vendor,
			Name:       name,
			Integrated: vendor == "apple" || vendor == "intel",
		})
	}
	if len(gpus) == 0 {
		// system_profiler failed or reported nothing; VideoToolbox still exists
		// on every supported macOS, so expose the synthetic default.
		gpus = append(gpus, GPUDevice{
			Vendor:     "apple",
			Name:       "Apple GPU",
			Integrated: true,
		})
	}
	return gpus
}

func darwinGPUVendor(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "apple"):
		return "apple"
	case strings.Contains(lower, "nvidia") || strings.Contains(lower, "geforce"):
		return "nvidia"
	case strings.Contains(lower, "amd") || strings.Contains(lower, "radeon") || strings.Contains(lower, "ati"):
		return "amd"
	case strings.Contains(lower, "intel"):
		return "intel"
	default:
		return "apple"
	}
}

package platform

import (
	"runtime"
	"strings"
)

// listWindowsGPUs enumerates display adapters via WMI (enumeration order; may differ from CUDA order).
func listWindowsGPUs() []GPUDevice {
	if runtime.GOOS != "windows" {
		return nil
	}
	out, err := combinedOutputTimeout("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-CimInstance Win32_VideoController | ForEach-Object { $_.PNPDeviceID + '|' + $_.Name }")
	if err != nil {
		return nil
	}
	var gpus []GPUDevice
	idx := 0
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pnp, name, _ := strings.Cut(line, "|")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		vendor := vendorFromDescription(name + " " + pnp)
		if vendor == "" {
			vendor = vendorFromDescription(pnp)
		}
		gpu := GPUDevice{
			ID:          stableWindowsGPUID(pnp, name),
			Name:        name,
			Vendor:      vendor,
			DXGIIndex:   idx,
			PNPDeviceID: strings.TrimSpace(pnp),
			Integrated:  classifyWindowsIntegrated(name, pnp),
		}
		gpus = append(gpus, gpu)
		idx++
	}
	return gpus
}

func stableWindowsGPUID(pnp, name string) string {
	pnp = strings.TrimSpace(pnp)
	if pnp != "" {
		return "win:" + pnp
	}
	return "win:name:" + strings.ToLower(strings.TrimSpace(name))
}

func classifyWindowsIntegrated(name, pnp string) bool {
	lower := strings.ToLower(name + " " + pnp)
	if strings.Contains(lower, "nvidia") && strings.Contains(lower, "geforce") {
		return false
	}
	if strings.Contains(lower, "radeon rx") {
		return false
	}
	if strings.Contains(lower, "intel") {
		return true
	}
	if strings.Contains(lower, "amd") || strings.Contains(lower, "radeon") {
		return strings.Contains(lower, "graphics") && !strings.Contains(lower, "rx ")
	}
	return false
}

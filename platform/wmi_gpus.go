package platform

import (
	"runtime"
	"strings"
	"sync"
	"time"
)

// windowsGPUCacheTTL bounds how long a WMI enumeration is reused. Detection
// and device listing each ask several times per start, and every query spawns
// PowerShell.
const windowsGPUCacheTTL = 30 * time.Second

var windowsGPUCache struct {
	mu   sync.Mutex
	at   time.Time
	gpus []GPUDevice
}

// listWindowsGPUs enumerates display adapters via WMI (enumeration order; may differ from CUDA order).
func listWindowsGPUs() []GPUDevice {
	if runtime.GOOS != "windows" {
		return nil
	}
	windowsGPUCache.mu.Lock()
	defer windowsGPUCache.mu.Unlock()
	if !windowsGPUCache.at.IsZero() && time.Since(windowsGPUCache.at) < windowsGPUCacheTTL {
		return append([]GPUDevice(nil), windowsGPUCache.gpus...)
	}
	gpus := queryWindowsGPUs()
	windowsGPUCache.at, windowsGPUCache.gpus = time.Now(), gpus
	return append([]GPUDevice(nil), gpus...)
}

func queryWindowsGPUs() []GPUDevice {
	out, err := combinedOutputTimeout("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-CimInstance Win32_VideoController | ForEach-Object { $_.PNPDeviceID + '|' + $_.Name }")
	if err != nil {
		return nil
	}
	gpus := parseWindowsAdapterList(string(out))
	for i := range gpus {
		gpus[i].Pinning = BackendPinning{
			D3D11: PinningBestEffort,
			AMF:   PinningBestEffort,
			QSV:   PinningBestEffort,
		}
	}
	return gpus
}

// parseWindowsAdapterList parses "PNPDeviceID|Name" rows emitted by
// Get-CimInstance Win32_VideoController — shared by Windows WMI and WSL
// powershell.exe interop.
func parseWindowsAdapterList(out string) []GPUDevice {
	var gpus []GPUDevice
	idx := 0
	for _, line := range strings.Split(out, "\n") {
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
		pnp = strings.TrimSpace(pnp)
		gpu := GPUDevice{
			ID:          stableWindowsGPUID(pnp, name),
			Name:        name,
			Vendor:      vendor,
			DXGIIndex:   idx,
			PNPDeviceID: pnp,
			PCIAddress:  PCIBusIDFromPNPDeviceID(pnp),
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

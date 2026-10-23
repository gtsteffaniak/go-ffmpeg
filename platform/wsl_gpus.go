package platform

import (
	"runtime"
	"sync"
	"time"
)

// WSL's lspci/DRM views only expose devices the paravirtualization channel
// registered (and only NVIDIA reliably), so Intel/AMD adapters are invisible
// inside the guest. The Windows host's own enumeration is reachable through
// powershell.exe interop on WSL2.
var wslGPUCache struct {
	mu   sync.Mutex
	at   time.Time
	gpus []GPUDevice
}

// listWSLGPUs enumerates the Windows host's display adapters from inside WSL.
// Returns nil when interop is unavailable so callers fall back to guest probes.
func listWSLGPUs() []GPUDevice {
	if runtime.GOOS != "linux" || !isWSLEnvironment() {
		return nil
	}
	wslGPUCache.mu.Lock()
	defer wslGPUCache.mu.Unlock()
	if !wslGPUCache.at.IsZero() && time.Since(wslGPUCache.at) < windowsGPUCacheTTL {
		return append([]GPUDevice(nil), wslGPUCache.gpus...)
	}
	out, err := combinedOutputTimeout("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"Get-CimInstance Win32_VideoController | ForEach-Object { $_.PNPDeviceID + '|' + $_.Name }")
	var gpus []GPUDevice
	if err == nil {
		gpus = parseWindowsAdapterList(string(out))
	}
	wslGPUCache.at, wslGPUCache.gpus = time.Now(), gpus
	return append([]GPUDevice(nil), gpus...)
}

// wslVendorPresent reports whether a successfully enumerated WSL adapter list
// contains the given vendor. Returns the fallback when enumeration produced no
// adapters (interop disabled) so callers can defer to guest-side probes.
func wslVendorPresent(vendor string, fallback func() bool) bool {
	gpus := listWSLGPUs()
	if len(gpus) == 0 {
		return fallback()
	}
	for _, g := range gpus {
		if g.Vendor == vendor {
			return true
		}
	}
	return false
}

package platform

import (
	"os"
	"runtime"
	"strings"
)

func amdGPUPresent() bool {
	switch runtime.GOOS {
	case "windows":
		return amdWindowsPresent()
	case "linux":
		return amdLinuxPresent()
	default:
		return false
	}
}

func amdLinuxPresent() bool {
	if drmVendorPresent("0x1002") {
		return true
	}
	if commandAvailable("lspci") {
		out, err := combinedOutputTimeout("lspci", "-nn")
		if err == nil {
			s := strings.ToLower(string(out))
			if strings.Contains(s, "[1002:") || strings.Contains(s, "amd/ati") {
				return true
			}
		}
	}
	return false
}

func amdWindowsPresent() bool {
	for _, path := range []string{
		`C:\Windows\System32\amfrt64.dll`,
		`C:\Windows\System32\amdxx64.dll`,
		`C:\Windows\System32\atidxx64.dll`,
	} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	if runSilentTimeout("reg", "query", `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`, "/s", "/f", "AMD") == nil {
		return true
	}
	if runSilentTimeout("reg", "query", `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`, "/s", "/f", "Radeon") == nil {
		return true
	}
	for _, g := range listWindowsGPUs() {
		if g.Vendor == "amd" {
			return true
		}
	}
	return false
}

// PickAMDGPUDevice returns the preferred AMD GPU device for AMF smoke tests.
func PickAMDGPUDevice(gpus []GPUDevice) *GPUDevice {
	var igpu, dgpu *GPUDevice
	for i := range gpus {
		g := &gpus[i]
		if g.Vendor != "amd" {
			continue
		}
		if g.Integrated {
			igpu = g
		} else {
			dgpu = g
		}
	}
	if dgpu != nil {
		return dgpu
	}
	return igpu
}

// PickNVIDIAGPUDevice returns the preferred NVIDIA GPU for NVENC smoke tests.
func PickNVIDIAGPUDevice(gpus []GPUDevice) *GPUDevice {
	for i := range gpus {
		g := &gpus[i]
		if g.Vendor == "nvidia" && !g.Integrated {
			return g
		}
	}
	for i := range gpus {
		if gpus[i].Vendor == "nvidia" {
			return &gpus[i]
		}
	}
	return nil
}

// PickIntelIGPUDevice returns an Intel integrated GPU render node when present.
func PickIntelIGPUDevice(gpus []GPUDevice) *GPUDevice {
	for i := range gpus {
		g := &gpus[i]
		if g.Vendor == "intel" && g.Integrated {
			return g
		}
	}
	for i := range gpus {
		if gpus[i].Vendor == "intel" {
			return &gpus[i]
		}
	}
	return nil
}

func nvidiaCUDADeviceIndex(pciAddr string) int {
	if pciAddr == "" || !commandAvailable("nvidia-smi") {
		return 0
	}
	out, err := combinedOutputTimeout("nvidia-smi", "--query-gpu=pci.bus_id", "--format=csv,noheader")
	if err != nil {
		return 0
	}
	want := strings.ToLower(strings.TrimSpace(pciAddr))
	idx := 0
	for _, line := range strings.Split(string(out), "\n") {
		bus := strings.ToLower(strings.TrimSpace(line))
		if bus == "" {
			continue
		}
		if strings.Contains(bus, want) || strings.Contains(want, bus) {
			return idx
		}
		idx++
	}
	return 0
}

func mergeGPULists(primary, extra []GPUDevice) []GPUDevice {
	seen := map[string]struct{}{}
	var out []GPUDevice
	for _, g := range primary {
		if g.ID == "" {
			g.ID = gpuStableID(g)
		}
		if _, ok := seen[g.ID]; ok {
			continue
		}
		seen[g.ID] = struct{}{}
		out = append(out, g)
	}
	for _, g := range extra {
		if g.ID == "" {
			g.ID = gpuStableID(g)
		}
		if _, ok := seen[g.ID]; ok {
			continue
		}
		seen[g.ID] = struct{}{}
		out = append(out, g)
	}
	return out
}

func gpuStableID(g GPUDevice) string {
	if g.RenderNode != "" {
		return "drm:" + g.RenderNode
	}
	if g.PNPDeviceID != "" {
		return "win:" + g.PNPDeviceID
	}
	if g.Name != "" {
		return "name:" + strings.ToLower(g.Name)
	}
	return ""
}

// FillGPUChoiceDevice pins adapter/render node fields on a resolved choice.
func FillGPUChoiceDevice(choice *GPUChoice, devices []GPUDevice) {
	if choice == nil || !choice.Enabled {
		return
	}
	for _, d := range devices {
		if choice.DeviceID != "" && d.ID == choice.DeviceID {
			applyDeviceToChoice(choice, d)
			return
		}
		if choice.RenderDevice != "" && d.RenderNode == choice.RenderDevice {
			applyDeviceToChoice(choice, d)
			return
		}
		if nameMatches(d.Name, choice.Name) && (choice.Vendor == "" || d.Vendor == choice.Vendor) {
			applyDeviceToChoice(choice, d)
			return
		}
	}
}

func applyDeviceToChoice(choice *GPUChoice, d GPUDevice) {
	if d.RenderNode != "" {
		choice.RenderDevice = d.RenderNode
	}
	if d.Vendor != "" {
		choice.Vendor = d.Vendor
	}
	if d.Name != "" {
		choice.Name = d.Name
	}
	choice.DeviceID = d.ID
	choice.DXGIIndex = d.DXGIIndex
	choice.LUID = d.LUID
	choice.Integrated = d.Integrated
	if d.PCIAddress != "" {
		choice.CUDAIndex = nvidiaCUDADeviceIndex(d.PCIAddress)
	}
}

// ListGPUDevices returns concrete GPUs for admin UIs (DRM nodes on Linux, WMI on Windows).
func ListGPUDevices() ([]GPUDevice, error) {
	var gpus []GPUDevice
	if runtime.GOOS == "linux" {
		drm, err := listDRMGPUs()
		if err != nil {
			return nil, err
		}
		gpus = drm
	}
	if runtime.GOOS == "windows" {
		gpus = mergeGPULists(gpus, listWindowsGPUs())
	}
	if !hasNVIDIAGPUDevice(gpus) && nvidiaAvailable() {
		g := syntheticNVIDIAGPU()
		g.ID = gpuStableID(g)
		if runtime.GOOS == "windows" {
			for _, wg := range listWindowsGPUs() {
				if wg.Vendor == "nvidia" {
					g = wg
					break
				}
			}
		}
		gpus = mergeGPULists(gpus, []GPUDevice{g})
	}
	for i := range gpus {
		if gpus[i].ID == "" {
			gpus[i].ID = gpuStableID(gpus[i])
		}
		if gpus[i].DeviceClass == "" {
			gpus[i].DeviceClass = deviceClassFor(gpus[i])
		}
	}
	return gpus, nil
}

func deviceClassFor(g GPUDevice) string {
	if g.Integrated {
		return "igpu"
	}
	if g.Vendor == "nvidia" || g.Vendor == "amd" {
		return "dgpu"
	}
	model := DetectPiModel()
	if model != PiUnknown {
		return "soc"
	}
	return "dgpu"
}

// DetectDevices is an alias for ListGPUDevices.
func DetectDevices() ([]GPUDevice, error) {
	return ListGPUDevices()
}

// Remove duplicate amdGPUPresent from detect.go - I need to update detect.go to remove old function

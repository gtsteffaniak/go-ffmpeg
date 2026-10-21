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
	if choice.DeviceID != "" {
		for _, d := range devices {
			if d.ID == choice.DeviceID {
				applyDeviceToChoice(choice, d)
				return
			}
		}
	}
	if choice.RenderDevice != "" {
		for _, d := range devices {
			if d.RenderNode == choice.RenderDevice {
				applyDeviceToChoice(choice, d)
				return
			}
		}
	}
	for _, d := range devices {
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
		choice.PCIAddress = d.PCIAddress
	}
	choice.Pinning = d.Pinning
	if d.Vendor == "nvidia" {
		if d.Pinning.NVENC != "" {
			choice.Pinning.NVENC = d.Pinning.NVENC
			choice.Pinning.CUDA = d.Pinning.CUDA
		}
		if d.CUDAIndex >= 0 && (d.Pinning.NVENC == PinningExact || d.Pinning.CUDA == PinningExact) {
			choice.CUDAIndex = d.CUDAIndex
		} else {
			table := QueryNVIDIAGPUIndexTable()
			pci := d.PCIAddress
			if pci == "" && d.PNPDeviceID != "" {
				pci = PCIBusIDFromPNPDeviceID(d.PNPDeviceID)
			}
			if idx, ok := MatchNVIDIACUDAIndex(pci, d.Name, table); ok {
				choice.CUDAIndex = idx
				choice.Pinning.NVENC = PinningExact
				choice.Pinning.CUDA = PinningExact
			} else if len(table) == 1 {
				choice.CUDAIndex = table[0].Index
				choice.Pinning.NVENC = PinningBestEffort
				choice.Pinning.CUDA = PinningBestEffort
			}
		}
	}
	if d.Pinning.D3D11 != "" {
		choice.Pinning.D3D11 = d.Pinning.D3D11
		choice.Pinning.AMF = d.Pinning.AMF
		choice.Pinning.QSV = d.Pinning.QSV
	}
	if choice.Pinning.D3D11 == "" {
		if d.DXGIFromEnum {
			choice.Pinning.D3D11 = PinningExact
			choice.Pinning.AMF = PinningExact
			choice.Pinning.QSV = PinningExact
		} else if runtime.GOOS == "windows" && d.Vendor != "" {
			choice.Pinning.D3D11 = PinningBestEffort
			choice.Pinning.AMF = PinningBestEffort
			choice.Pinning.QSV = PinningBestEffort
			if choice.Pinning.Note == "" {
				choice.Pinning.Note = "D3D11VA/QSV/AMF child_device uses DXGI adapter index; WMI order may differ from DXGI enumeration"
			}
		}
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
	if runtime.GOOS == "darwin" {
		gpus = mergeGPULists(gpus, listDarwinGPUs())
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
	if runtime.GOOS == "windows" {
		enrichWindowsGPUs(gpus)
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

package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type dxgiAdapterRow struct {
	Index int
	LUID  string
	Name  string
}

func enrichWindowsGPUs(gpus []GPUDevice) {
	if runtime.GOOS != "windows" || len(gpus) == 0 {
		return
	}
	for i := range gpus {
		if gpus[i].PCIAddress == "" && gpus[i].PNPDeviceID != "" {
			gpus[i].PCIAddress = PCIBusIDFromPNPDeviceID(gpus[i].PNPDeviceID)
		}
	}
	annotateNVIDIAPCI(gpus)
	mergeDXGIAdapters(gpus)
}

func annotateNVIDIAPCI(gpus []GPUDevice) {
	table := QueryNVIDIAGPUIndexTable()
	for i := range gpus {
		if gpus[i].Vendor != "nvidia" {
			continue
		}
		pci := gpus[i].PCIAddress
		if idx, ok := MatchNVIDIACUDAIndex(pci, gpus[i].Name, table); ok {
			gpus[i].CUDAIndex = idx
			gpus[i].Pinning.NVENC = PinningExact
			gpus[i].Pinning.CUDA = PinningExact
		}
	}
}

func mergeDXGIAdapters(gpus []GPUDevice) {
	rows := queryDXGIAdapters()
	if len(rows) == 0 {
		return
	}
	for i := range gpus {
		for _, row := range rows {
			if nameMatches(gpus[i].Name, row.Name) || strings.Contains(strings.ToLower(row.Name), strings.ToLower(gpus[i].Vendor)) && nameMatches(gpus[i].Name, row.Name) {
				gpus[i].DXGIIndex = row.Index
				gpus[i].LUID = row.LUID
				gpus[i].DXGIFromEnum = true
				if gpus[i].Pinning.D3D11 == "" {
					gpus[i].Pinning.D3D11 = PinningExact
					gpus[i].Pinning.AMF = PinningExact
					gpus[i].Pinning.QSV = PinningExact
				}
				break
			}
		}
	}
}

func queryDXGIAdapters() []dxgiAdapterRow {
	script := dxgiScriptPath()
	if script == "" {
		return nil
	}
	out, err := combinedOutputTimeout("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script)
	if err != nil {
		return nil
	}
	var rows []dxgiAdapterRow
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 3)
		if len(parts) < 3 {
			continue
		}
		idx, err := parseInt(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		rows = append(rows, dxgiAdapterRow{
			Index: idx,
			LUID:  strings.TrimSpace(parts[1]),
			Name:  strings.TrimSpace(parts[2]),
		})
	}
	return rows
}

func dxgiScriptPath() string {
	if p := os.Getenv("GOFFMPEG_DXGI_SCRIPT"); p != "" {
		return p
	}
	// Relative to module root when running tests from repo.
	candidates := []string{
		"scripts/dxgi-adapters.ps1",
		filepath.Join("..", "scripts", "dxgi-adapters.ps1"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs
			}
			return c
		}
	}
	return ""
}

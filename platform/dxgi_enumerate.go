package platform

import (
	"os"
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
				gpus[i].Pinning.D3D11 = PinningExact
				gpus[i].Pinning.AMF = PinningExact
				gpus[i].Pinning.QSV = PinningExact
				break
			}
		}
	}
}

func queryDXGIAdapters() []dxgiAdapterRow {
	scriptPath, cleanup, err := dxgiScriptFile()
	if scriptPath == "" || err != nil {
		return nil
	}
	if cleanup != nil {
		defer cleanup()
	}
	out, err := combinedOutputTimeout("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
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

// dxgiScriptFile returns a PowerShell script path and optional cleanup for a temp file.
func dxgiScriptFile() (path string, cleanup func(), err error) {
	if p := os.Getenv("GOFFMPEG_DXGI_SCRIPT"); p != "" {
		return p, nil, nil
	}
	if embeddedDXGIAdapters == "" {
		return "", nil, nil
	}
	f, err := os.CreateTemp("", "goffmpeg-dxgi-*.ps1")
	if err != nil {
		return "", nil, err
	}
	if _, err := f.WriteString(embeddedDXGIAdapters); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

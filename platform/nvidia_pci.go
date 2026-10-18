package platform

import (
	"strings"
)

// NVIDIAGPUIndex row from nvidia-smi --query-gpu=index,pci.bus_id,name
type NVIDIAGPUIndex struct {
	Index    int
	PCIBusID string
	Name     string
}

// NormalizePCIBusID canonicalizes PCI bus IDs for comparison (domain:bus:device.function).
func NormalizePCIBusID(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimPrefix(raw, "pci\\")
	raw = strings.ReplaceAll(raw, "\\", "")
	if raw == "" {
		return ""
	}
	// Already domain:bb:dd.f
	if strings.Contains(raw, ":") && strings.Contains(raw, ".") {
		parts := strings.Split(raw, ":")
		if len(parts) >= 3 {
			domain := parts[0]
			bus := parts[1]
			devFunc := parts[2]
			if len(domain) < 8 {
				domain = strings.Repeat("0", 8-len(domain)) + domain
			}
			return domain + ":" + bus + ":" + devFunc
		}
		return raw
	}
	return raw
}

// PCIBusIDFromPNPDeviceID extracts a PCI location from a Windows PNP device id when possible.
func PCIBusIDFromPNPDeviceID(pnp string) string {
	pnp = strings.TrimSpace(pnp)
	if pnp == "" {
		return ""
	}
	_ = strings.ToLower(pnp)
	// Windows PNP instance suffixes are not PCI bus:device.function; match by name or location info instead.
	return ""
}

// ParseNVIDIASMIIndexCSV parses nvidia-smi csv output (index, pci.bus_id[, name]).
func ParseNVIDIASMIIndexCSV(output string) []NVIDIAGPUIndex {
	var out []NVIDIAGPUIndex
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		idx, err := parseInt(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		pci := NormalizePCIBusID(strings.TrimSpace(parts[1]))
		name := ""
		if len(parts) >= 3 {
			name = strings.TrimSpace(parts[2])
		}
		out = append(out, NVIDIAGPUIndex{Index: idx, PCIBusID: pci, Name: name})
	}
	return out
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errInvalidInt
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

var errInvalidInt = stringsErr("invalid int")

type stringsErr string

func (e stringsErr) Error() string { return string(e) }

// MatchNVIDIACUDAIndex finds the CUDA device index for a PCI bus id or GPU name.
func MatchNVIDIACUDAIndex(pciBusID, gpuName string, table []NVIDIAGPUIndex) (int, bool) {
	wantPCI := NormalizePCIBusID(pciBusID)
	if wantPCI != "" {
		for _, row := range table {
			if PCIEqual(row.PCIBusID, wantPCI) {
				return row.Index, true
			}
		}
	}
	if gpuName != "" {
		for _, row := range table {
			if nameMatches(row.Name, gpuName) || nameMatches(gpuName, row.Name) {
				return row.Index, true
			}
		}
	}
	return 0, false
}

// PCIEqual compares PCI bus ids allowing domain padding differences.
func PCIEqual(a, b string) bool {
	a = NormalizePCIBusID(a)
	b = NormalizePCIBusID(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	stripDomain := func(s string) string {
		if strings.Count(s, ":") >= 2 {
			return s[strings.Index(s, ":")+1:]
		}
		return s
	}
	return stripDomain(a) == stripDomain(b)
}

// QueryNVIDIAGPUIndexTable runs nvidia-smi when available.
func QueryNVIDIAGPUIndexTable() []NVIDIAGPUIndex {
	if !commandAvailable("nvidia-smi") {
		return nil
	}
	out, err := combinedOutputTimeout("nvidia-smi", "--query-gpu=index,pci.bus_id,name", "--format=csv,noheader")
	if err != nil {
		out, err = combinedOutputTimeout("nvidia-smi", "--query-gpu=index,pci.bus_id", "--format=csv,noheader")
		if err != nil {
			return nil
		}
	}
	return ParseNVIDIASMIIndexCSV(string(out))
}

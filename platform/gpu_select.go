package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// GPUChoice describes a resolved GPU selection from user config.
type GPUChoice struct {
	Enabled      bool
	RenderDevice string
	Vendor       string // nvidia, intel, amd, unknown
	Name         string
}

type drmGPU struct {
	RenderNode  string
	VendorID    string
	Vendor      string
	Name        string
	Integrated  bool
	PCIAddress  string
}

// ResolveGPU interprets a gpu config value.
// Empty input disables hardware acceleration.
// Supported values: default, igpu, dgpu, a render node path (/dev/dri/renderD*), or a device name substring.
func ResolveGPU(spec string) (GPUChoice, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return GPUChoice{}, nil
	}

	switch runtime.GOOS {
	case "linux":
		return resolveGPULinux(spec)
	case "darwin":
		return GPUChoice{Enabled: true, Vendor: "apple", Name: "VideoToolbox"}, nil
	case "windows":
		return resolveGPUWindows(spec)
	default:
		return GPUChoice{Enabled: true, Vendor: "unknown", Name: spec}, nil
	}
}

func resolveGPULinux(spec string) (GPUChoice, error) {
	devices, err := listDRMGPUs()
	if err != nil {
		return GPUChoice{}, err
	}

	key := strings.ToLower(spec)
	switch key {
	case "default", "igpu":
		for _, d := range devices {
			if d.Integrated {
				return gpuChoiceFromDRM(d), nil
			}
		}
		if len(devices) > 0 {
			return gpuChoiceFromDRM(devices[0]), nil
		}
		return GPUChoice{}, fmt.Errorf("no integrated GPU render node found for gpu=%q", spec)
	case "dgpu":
		for _, d := range devices {
			if !d.Integrated {
				return gpuChoiceFromDRM(d), nil
			}
		}
		return GPUChoice{}, fmt.Errorf("no discrete GPU render node found for gpu=%q", spec)
	}

	if strings.HasPrefix(spec, "/dev/") {
		if _, err := os.Stat(spec); err != nil {
			return GPUChoice{}, fmt.Errorf("gpu render node %q not accessible: %w", spec, err)
		}
		for _, d := range devices {
			if d.RenderNode == spec {
				return gpuChoiceFromDRM(d), nil
			}
		}
		vendor := vendorFromRenderNode(spec)
		return GPUChoice{
			Enabled:      true,
			RenderDevice: spec,
			Vendor:       vendor,
			Name:         spec,
		}, nil
	}

	for _, d := range devices {
		if nameMatches(d.Name, spec) || nameMatches(d.RenderNode, spec) {
			return gpuChoiceFromDRM(d), nil
		}
	}
	if desc := detectGPUDescription(); desc != "" {
		for _, part := range strings.Split(desc, ";") {
			part = strings.TrimSpace(part)
			if nameMatches(part, spec) {
				vendor := vendorFromDescription(part)
				for _, d := range devices {
					if d.Vendor == vendor || nameMatches(d.Name, part) {
						return gpuChoiceFromDRM(d), nil
					}
				}
				return GPUChoice{Enabled: true, Vendor: vendor, Name: part}, nil
			}
		}
	}
	return GPUChoice{}, fmt.Errorf("no GPU matching %q", spec)
}

func resolveGPUWindows(spec string) (GPUChoice, error) {
	key := strings.ToLower(strings.TrimSpace(spec))
	switch key {
	case "default", "igpu", "dgpu":
		if nvidiaAvailable() {
			return GPUChoice{Enabled: true, Vendor: "nvidia", Name: "NVIDIA GPU"}, nil
		}
		if intelGPUPresent() {
			return GPUChoice{Enabled: true, Vendor: "intel", Name: "Intel GPU"}, nil
		}
		if amdGPUPresent() {
			return GPUChoice{Enabled: true, Vendor: "amd", Name: "AMD GPU"}, nil
		}
		return GPUChoice{}, fmt.Errorf("no GPU found for gpu=%q", spec)
	default:
		desc := detectGPUDescription()
		if desc != "" && nameMatches(desc, spec) {
			return GPUChoice{Enabled: true, Vendor: vendorFromDescription(desc), Name: spec}, nil
		}
		if nvidiaAvailable() && nameMatches("nvidia", spec) {
			return GPUChoice{Enabled: true, Vendor: "nvidia", Name: spec}, nil
		}
		return GPUChoice{Enabled: true, Vendor: "unknown", Name: spec}, nil
	}
}

func gpuChoiceFromDRM(d drmGPU) GPUChoice {
	return GPUChoice{
		Enabled:      true,
		RenderDevice: d.RenderNode,
		Vendor:       d.Vendor,
		Name:         d.Name,
	}
}

func listDRMGPUs() ([]drmGPU, error) {
	matches, err := filepath.Glob("/dev/dri/renderD*")
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	out := make([]drmGPU, 0, len(matches))
	for _, node := range matches {
		if _, err := os.Stat(node); err != nil {
			continue
		}
		gpu := drmGPU{
			RenderNode: node,
			VendorID:   readDRMVendorID(node),
			PCIAddress: readDRMPciAddress(node),
			Name:       readDRMDeviceName(node),
		}
		gpu.Vendor = vendorFromID(gpu.VendorID)
		gpu.Integrated = classifyIntegrated(gpu)
		if gpu.Name == "" {
			gpu.Name = gpu.RenderNode
		}
		out = append(out, gpu)
	}
	return out, nil
}

func classifyIntegrated(g drmGPU) bool {
	switch strings.ToLower(g.VendorID) {
	case "0x10de":
		return false
	case "0x1002":
		return isAMDIntegratedPCI(g.PCIAddress, g.Name)
	case "0x8086":
		return isIntelIntegratedPCI(g.PCIAddress, g.Name)
	default:
		return isIntegratedPCIAddress(g.PCIAddress)
	}
}

func isIntelIntegratedPCI(pciAddr, name string) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "arc ") && !strings.Contains(lower, "integrated") {
		return false
	}
	if strings.Contains(lower, "hd graphics") || strings.Contains(lower, "uhd graphics") ||
		strings.Contains(lower, "iris") || strings.Contains(lower, "xe graphics") {
		return true
	}
	return isIntegratedPCIAddress(pciAddr)
}

func isAMDIntegratedPCI(pciAddr, name string) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "radeon rx") || strings.Contains(lower, "rx ") {
		return false
	}
	return isIntegratedPCIAddress(pciAddr)
}

func isIntegratedPCIAddress(pciAddr string) bool {
	pciAddr = strings.TrimSpace(pciAddr)
	if pciAddr == "" {
		return false
	}
	parts := strings.Split(pciAddr, ":")
	if len(parts) < 2 {
		return false
	}
	devFunc := parts[len(parts)-1]
	return devFunc == "02.0" || devFunc == "02.1" || strings.HasPrefix(devFunc, "05.")
}

func readDRMVendorID(renderNode string) string {
	base := drmSysfsBase(renderNode)
	if base == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(base, "device", "vendor"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readDRMPciAddress(renderNode string) string {
	base := drmSysfsBase(renderNode)
	if base == "" {
		return ""
	}
	uevent, err := os.ReadFile(filepath.Join(base, "device", "uevent"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(uevent), "\n") {
		if strings.HasPrefix(line, "PCI_SLOT_NAME=") {
			return strings.TrimPrefix(line, "PCI_SLOT_NAME=")
		}
	}
	return ""
}

func readDRMDeviceName(renderNode string) string {
	if commandAvailable("lspci") {
		pciAddr := readDRMPciAddress(renderNode)
		if pciAddr == "" {
			return ""
		}
		out, err := exec.Command("lspci", "-s", pciAddr).CombinedOutput()
		if err == nil {
			line := strings.TrimSpace(string(out))
			if parts := strings.SplitN(line, " ", 2); len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
			return line
		}
	}
	return renderNode
}

func drmSysfsBase(renderNode string) string {
	name := filepath.Base(renderNode)
	path := filepath.Join("/sys/class/drm", name)
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func vendorFromRenderNode(renderNode string) string {
	return vendorFromID(readDRMVendorID(renderNode))
}

func vendorFromID(vendorID string) string {
	switch strings.ToLower(vendorID) {
	case "0x10de":
		return "nvidia"
	case "0x8086":
		return "intel"
	case "0x1002":
		return "amd"
	default:
		return "unknown"
	}
}

func vendorFromDescription(desc string) string {
	lower := strings.ToLower(desc)
	switch {
	case strings.Contains(lower, "nvidia"):
		return "nvidia"
	case strings.Contains(lower, "intel"):
		return "intel"
	case strings.Contains(lower, "amd") || strings.Contains(lower, "ati"):
		return "amd"
	default:
		return "unknown"
	}
}

func nameMatches(haystack, needle string) bool {
	haystack = strings.ToLower(strings.TrimSpace(haystack))
	needle = strings.ToLower(strings.TrimSpace(needle))
	if haystack == "" || needle == "" {
		return false
	}
	return strings.Contains(haystack, needle)
}

// NameMatchesForTest exposes name matching for unit tests.
func NameMatchesForTest(haystack, needle string) bool {
	return nameMatches(haystack, needle)
}

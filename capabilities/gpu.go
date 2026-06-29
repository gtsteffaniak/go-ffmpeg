package capabilities

import "github.com/gtsteffaniak/go-ffmpeg/platform"

// HierarchyForGPU returns encoder preference order for a selected GPU vendor.
// Returns nil to fall back to HierarchyForPlatform.
func HierarchyForGPU(vendor string, plat PlatformInfo) []AccelType {
	switch vendor {
	case "nvidia":
		return []AccelType{AccelNVENC}
	case "intel":
		if plat.WSL {
			return []AccelType{AccelNVENC, AccelD3D12, AccelVAAPI, AccelQSV}
		}
		return []AccelType{AccelQSV, AccelVAAPI}
	case "amd":
		return []AccelType{AccelAMF, AccelVAAPI}
	case "apple":
		return []AccelType{AccelVideoToolbox}
	default:
		return nil
	}
}

// ResolveGPUChoice resolves a gpu config string using platform detection.
func ResolveGPUChoice(spec string) (platform.GPUChoice, error) {
	return platform.ResolveGPU(spec)
}

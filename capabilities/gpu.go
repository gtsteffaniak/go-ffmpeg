package capabilities

// HierarchyForGPU returns encoder preference order for a selected GPU vendor.
// Returns nil to fall back to HierarchyForPlatform.
func HierarchyForGPU(vendor string, plat PlatformInfo) []AccelType {
	switch vendor {
	case "nvidia":
		return []AccelType{AccelNVENC}
	case "intel":
		if plat.OS == "windows" {
			return []AccelType{AccelQSV}
		}
		if plat.WSL {
			return []AccelType{AccelVAAPI, AccelQSV, AccelD3D12, AccelNVENC}
		}
		return []AccelType{AccelVAAPI, AccelQSV}
	case "amd":
		if plat.OS == "windows" {
			return []AccelType{AccelAMF}
		}
		return []AccelType{AccelAMF, AccelVAAPI}
	case "apple":
		return []AccelType{AccelVideoToolbox}
	case "soc":
		return []AccelType{AccelV4L2M2M}
	default:
		return nil
	}
}

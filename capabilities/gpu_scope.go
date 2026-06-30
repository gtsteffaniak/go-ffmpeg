package capabilities

import "github.com/gtsteffaniak/go-ffmpeg/platform"

func encoderKindToAccel(kind string) AccelType {
	switch kind {
	case "nvenc":
		return AccelNVENC
	case "amf":
		return AccelAMF
	case "qsv":
		return AccelQSV
	case "vaapi":
		return AccelVAAPI
	case "videotoolbox":
		return AccelVideoToolbox
	default:
		return ""
	}
}

func encoderKindInHierarchy(kind string, hierarchy []AccelType) bool {
	accel := encoderKindToAccel(kind)
	if accel == "" {
		return false
	}
	for _, h := range hierarchy {
		if h == accel {
			return true
		}
		if h == AccelD3D12 && kind == "vaapi" {
			return true
		}
	}
	return false
}

func hierarchyIncludesAccel(hierarchy []AccelType, accel AccelType) bool {
	for _, h := range hierarchy {
		if h == accel {
			return true
		}
	}
	return false
}

func gpuScopedSkipReason(plat platform.Info) string {
	if name := plat.Details["selected_gpu"]; name != "" {
		return "[gpu] not applicable to selected GPU (" + name + ")"
	}
	return "[gpu] not applicable to selected GPU"
}

// EncoderKindInHierarchyForTest exposes hierarchy matching for unit tests.
func EncoderKindInHierarchyForTest(kind string, hierarchy []AccelType) bool {
	return encoderKindInHierarchy(kind, hierarchy)
}

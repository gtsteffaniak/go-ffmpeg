package capabilities

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// EncoderPreference selects hardware encoder backend priority when EncoderHierarchy is unset.
type EncoderPreference string

const (
	// EncoderPreferenceDefault uses HierarchyForPlatform (VAAPI on Linux, QSV on Windows).
	EncoderPreferenceDefault EncoderPreference = ""
	// EncoderPreferencePowerSave matches the platform iGPU-first default (alias for clarity).
	EncoderPreferencePowerSave EncoderPreference = "powersave"
	// EncoderPreferencePerformance prefers discrete GPU backends (NVENC before VAAPI on Linux).
	EncoderPreferencePerformance EncoderPreference = "performance"
)

// ParseEncoderPreference validates a preference string from config.
func ParseEncoderPreference(s string) (EncoderPreference, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "", "default":
		return EncoderPreferenceDefault, nil
	case "performance", "dgpu", "dgpu-first":
		return EncoderPreferencePerformance, nil
	case "powersave", "power-save", "igpu", "igpu-first", "integrated":
		return EncoderPreferencePowerSave, nil
	default:
		return EncoderPreference(""), fmt.Errorf("unknown encoder preference %q (use default, powersave, or performance)", s)
	}
}

// HierarchyForPowerSave returns encoder order that favors integrated GPU backends on hybrid systems.
func HierarchyForPowerSave(plat PlatformInfo) []AccelType {
	return HierarchyForPlatform(plat)
}

// HierarchyForPerformance returns encoder order that favors discrete GPU backends when available.
func HierarchyForPerformance(plat PlatformInfo) []AccelType {
	switch plat.OS {
	case "darwin":
		return HierarchyForPlatform(plat)
	case "windows":
		return []AccelType{AccelNVENC, AccelQSV, AccelAMF}
	}
	if plat.WSL {
		return []AccelType{AccelNVENC, AccelD3D12, AccelVAAPI, AccelQSV}
	}
	return []AccelType{AccelNVENC, AccelVAAPI, AccelQSV, AccelAMF}
}

// ResolveEncoderHierarchy picks the effective encoder preference order.
// Precedence: explicit EncoderHierarchy, then GPU vendor scoping, then EncoderPreference, then platform default.
func ResolveEncoderHierarchy(explicit []AccelType, pref EncoderPreference, gpu platform.GPUChoice, plat PlatformInfo) []AccelType {
	if len(explicit) > 0 {
		return append([]AccelType(nil), explicit...)
	}
	if gpu.Enabled {
		if h := HierarchyForGPU(gpu.Vendor, plat); len(h) > 0 {
			return h
		}
	}
	switch pref {
	case EncoderPreferencePerformance:
		return HierarchyForPerformance(plat)
	case EncoderPreferencePowerSave:
		return HierarchyForPowerSave(plat)
	default:
		return HierarchyForPlatform(plat)
	}
}

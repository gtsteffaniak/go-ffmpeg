package capabilities

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// EncoderPreference selects hardware encoder backend priority when EncoderHierarchy is unset.
type EncoderPreference string

const (
	// EncoderPreferenceDefault uses HierarchyForPlatform (NVENC before VAAPI on Linux hybrid).
	EncoderPreferenceDefault EncoderPreference = ""
	// EncoderPreferencePowerSave prefers integrated GPU paths (VAAPI/QSV) over discrete GPUs.
	EncoderPreferencePowerSave EncoderPreference = "powersave"
)

// ParseEncoderPreference validates a preference string from config.
func ParseEncoderPreference(s string) (EncoderPreference, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "", "default", "performance", "dgpu", "dgpu-first":
		return EncoderPreferenceDefault, nil
	case "powersave", "power-save", "igpu", "igpu-first", "integrated":
		return EncoderPreferencePowerSave, nil
	default:
		return EncoderPreference(""), fmt.Errorf("unknown encoder preference %q (use default or powersave)", s)
	}
}

// HierarchyForPowerSave returns encoder order that favors integrated GPU backends on hybrid systems.
func HierarchyForPowerSave(plat PlatformInfo) []AccelType {
	switch plat.OS {
	case "darwin":
		return HierarchyForPlatform(plat)
	case "windows":
		return []AccelType{AccelQSV, AccelNVENC, AccelAMF}
	}
	if plat.WSL {
		return []AccelType{AccelVAAPI, AccelQSV, AccelD3D12, AccelNVENC}
	}
	return []AccelType{AccelVAAPI, AccelQSV, AccelNVENC, AccelAMF}
}

// ResolveEncoderHierarchy picks the effective encoder preference order.
// explicit wins, then GPU vendor scoping, then preference mode, then platform default.
func ResolveEncoderHierarchy(explicit []AccelType, pref EncoderPreference, gpu platform.GPUChoice, plat PlatformInfo) []AccelType {
	if len(explicit) > 0 {
		return append([]AccelType(nil), explicit...)
	}
	if gpu.Enabled {
		if h := HierarchyForGPU(gpu.Vendor, plat); len(h) > 0 {
			return h
		}
	}
	if pref == EncoderPreferencePowerSave {
		return HierarchyForPowerSave(plat)
	}
	return HierarchyForPlatform(plat)
}

package capabilities

import "strings"

// ParseAccelType parses a hardware acceleration backend name.
// Returns false when raw is not a known AccelType.
func ParseAccelType(raw string) (AccelType, bool) {
	a := AccelType(strings.ToLower(strings.TrimSpace(raw)))
	switch a {
	case AccelNVENC, AccelAMF, AccelQSV, AccelVAAPI, AccelD3D12, AccelVideoToolbox, AccelNone:
		return a, true
	default:
		return "", false
	}
}

// ParseAccelSelection interprets a user-facing acceleration setting.
// autoMode is true for empty input and "auto" (use platform default hierarchy).
// Returns AccelNone when software-only encoding is requested.
func ParseAccelSelection(raw string) (accel AccelType, autoMode bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "", "auto":
		return "", true
	case "none", "software", "false", "off", "disabled":
		return AccelNone, false
	default:
		if a, ok := ParseAccelType(s); ok {
			return a, false
		}
		return "", true
	}
}

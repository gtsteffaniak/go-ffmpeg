package capabilities

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a parsed ffmpeg semantic version (major.minor.patch).
type Version struct {
	Major int
	Minor int
	Patch int
}

// MinSupportedVersion is the hard minimum ffmpeg version for transcoding features.
var MinSupportedVersion = Version{Major: 5, Minor: 0, Patch: 0}

// FeatureFlags records ffmpeg version-gated capabilities.
type FeatureFlags struct {
	Version         Version `json:"version"`
	NoiseBSFDrop    bool    `json:"noiseBsfDrop"`
	Readrate        bool    `json:"readrate"` // true when version >= 5.0 (always true at MinSupportedVersion)
	ReadrateCatchup bool    `json:"readrateCatchup"`
	InputSideBSF    bool    `json:"inputSideBsf"`
}

// String formats a version as major.minor.patch.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// ParseSemver parses an ffmpeg version tag (e.g. "8.1.1", "n8.1.1", "5.0").
//
// Build-suffix tokens after the numeric version are tolerated in every
// component: Windows builds report versions like "9.0-full_build-www.gyan.dev",
// whose dot-separated tail contains non-numeric tokens ("gyan", "dev"). A
// component with no leading digits is treated as the end of the version
// (component 0) rather than a parse error, so "9.0-full_build-www.gyan.dev"
// parses as 9.0.0 instead of failing and failing the minimum-version gate.
func ParseSemver(raw string) (Version, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Version{}, fmt.Errorf("empty version")
	}
	// Strip leading non-digit prefix (nightly tags like n8.1.1).
	start := 0
	for start < len(raw) && (raw[start] < '0' || raw[start] > '9') {
		start++
	}
	if start >= len(raw) {
		return Version{}, fmt.Errorf("no numeric version in %q", raw)
	}
	parts := strings.Split(raw[start:], ".")
	if len(parts) < 1 {
		return Version{}, fmt.Errorf("invalid version %q", raw)
	}
	// leadingDigits returns the numeric prefix of s and whether s had one.
	leadingDigits := func(s string) (int, bool) {
		s = strings.TrimSpace(s)
		end := 0
		for end < len(s) && s[end] >= '0' && s[end] <= '9' {
			end++
		}
		if end == 0 {
			return 0, false
		}
		n, err := strconv.Atoi(s[:end])
		if err != nil {
			return 0, false
		}
		return n, true
	}
	major, ok := leadingDigits(parts[0])
	if !ok {
		return Version{}, fmt.Errorf("invalid version %q: invalid version component %q", raw, parts[0])
	}
	minor, patch := 0, 0
	if len(parts) > 1 {
		if n, ok := leadingDigits(parts[1]); ok {
			minor = n
		}
	}
	if len(parts) > 2 {
		if n, ok := leadingDigits(parts[2]); ok {
			patch = n
		}
	}
	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// Compare returns -1 if a < b, 0 if equal, 1 if a > b.
func Compare(a, b Version) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// FeatureFlagsFromVersion derives version-gated flags from a parsed version.
func FeatureFlagsFromVersion(v Version) FeatureFlags {
	return FeatureFlags{
		Version:         v,
		NoiseBSFDrop:    Compare(v, MinSupportedVersion) >= 0,
		Readrate:        Compare(v, Version{5, 0, 0}) >= 0,
		ReadrateCatchup: Compare(v, Version{8, 0, 0}) >= 0,
		InputSideBSF:    Compare(v, Version{7, 0, 0}) >= 0,
	}
}

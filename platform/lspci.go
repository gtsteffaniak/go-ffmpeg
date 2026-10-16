package platform

import (
	"regexp"
	"strings"
)

// pciDisplayClass matches lspci -nn display controller class 0x03xx.
var pciDisplayClass = regexp.MustCompile(`(?i)\[(03[0-9a-f]{2}):`)

// isDisplayControllerLine reports whether an lspci -nn line describes a GPU/display device.
func isDisplayControllerLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if pciDisplayClass.MatchString(line) {
		return true
	}
	lower := strings.ToLower(line)
	// Fallback when class codes are missing (rare).
	if strings.Contains(lower, "vga compatible controller") {
		return true
	}
	if strings.Contains(lower, "3d controller") {
		return true
	}
	if strings.Contains(lower, "display controller") {
		return true
	}
	return false
}

// IsDisplayControllerLineForTest exposes PCI display filtering for unit tests.
func IsDisplayControllerLineForTest(line string) bool {
	return isDisplayControllerLine(line)
}

// FilterDisplayControllerLines keeps only GPU/display PCI lines.
func FilterDisplayControllerLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		if isDisplayControllerLine(line) {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

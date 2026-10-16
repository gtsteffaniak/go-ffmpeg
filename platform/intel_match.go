package platform

import (
	"regexp"
	"strings"
)

var intelGPUMarkerPatterns = func() []*regexp.Regexp {
	phrases := []string{
		"graphics", "hd graphics", "uhd graphics", "iris", "arc",
		"lunar", "meteor", "arrow", "raptor",
	}
	out := make([]*regexp.Regexp, 0, len(phrases))
	for _, p := range phrases {
		// Word boundaries on the phrase ends; spaces inside multi-word phrases stay literal.
		pattern := `\b` + strings.ReplaceAll(regexp.QuoteMeta(p), " ", `\s+`) + `\b`
		out = append(out, regexp.MustCompile(pattern))
	}
	return out
}()

func lineHasIntelGPUMarker(line string) bool {
	lower := strings.ToLower(line)
	for _, re := range intelGPUMarkerPatterns {
		if re.MatchString(lower) {
			return true
		}
	}
	return false
}

func isDisplayPCILine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "vga") ||
		strings.Contains(lower, "3d") ||
		strings.Contains(lower, "display")
}

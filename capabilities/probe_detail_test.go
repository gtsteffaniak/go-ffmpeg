package capabilities

import (
	"strings"
	"testing"
)

func TestCompactProbeDetailNVENCDriverAPI(t *testing.T) {
	raw := "Input #0, lavfi, from 'testsrc=duration=0.5:size=320x240:rate=30':\n" +
		"Driver does not support the required nvenc API version. Required: 13.0 Found: 12.0\n" +
		"Error while opening encoder\n"
	detail := compactProbeDetail(raw, 240)
	if strings.Contains(strings.ToLower(detail), "lavfi") {
		t.Fatalf("detail should not be lavfi banner: %q", detail)
	}
	if !strings.Contains(detail, "nvenc API version") {
		t.Fatalf("detail = %q", detail)
	}
}
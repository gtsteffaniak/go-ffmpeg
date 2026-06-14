package capabilities

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestHWTroubleshootVP9VAAPI(t *testing.T) {
	output := `[vost#0:0/vp9_vaapi] Task finished with error code: -22 (Invalid argument)`
	primary, hints := hwTroubleshoot("vp9_vaapi", "vaapi", output, platform.Info{OS: "linux", Intel: true, VAAPI: true})
	if !strings.Contains(primary, "[hardware]") {
		t.Fatalf("primary = %q", primary)
	}
	if !strings.Contains(primary, "Linux") && !strings.Contains(primary, "Windows") {
		t.Fatalf("expected Linux/Windows note in primary, got %q", primary)
	}
	if len(hints) != 1 {
		t.Fatalf("expected one concise hint, got %v", hints)
	}
}

func TestHWTroubleshootVP9QSV(t *testing.T) {
	output := `[vp9_qsv] Current pixel format is unsupported`
	primary, hints := hwTroubleshoot("vp9_qsv", "qsv", output, platform.Info{OS: "linux", Intel: true, QSV: true, QSVRuntime: true, VAAPI: true})
	if !strings.Contains(primary, "Linux") {
		t.Fatalf("primary = %q", primary)
	}
	if len(hints) != 1 {
		t.Fatalf("expected one concise hint, got %v", hints)
	}
}

func TestHWTroubleshootQSV(t *testing.T) {
	output := `[QSV] Error creating a MFX session: -9.`
	primary, hints := hwTroubleshoot("h264_qsv", "qsv", output, platform.Info{Intel: true, QSV: true, VAAPI: true, QSVRuntime: false})
	if !strings.Contains(primary, "[driver]") {
		t.Fatalf("primary = %q", primary)
	}
	if len(hints) < 2 {
		t.Fatalf("expected multiple hints, got %v", hints)
	}
	if !strings.Contains(hints[0], "libmfx-gen1.2") {
		t.Fatalf("expected libmfx-gen hint, got %v", hints)
	}
}

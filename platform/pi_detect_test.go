package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPiModelFromDeviceTree(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model")
	if err := os.WriteFile(modelPath, []byte("Raspberry Pi 5 Model B Rev 1.0\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOFFMPEG_PI_MODEL", "")
	// Direct helper via env override for CI without sysfs
	t.Setenv("GOFFMPEG_PI_MODEL", "pi5")
	if DetectPiModel() != Pi5 {
		t.Fatalf("expected pi5")
	}
}

func TestPiEncodeProfilePi5SoftwareH264(t *testing.T) {
	p := PiEncodeProfileForModel(Pi5)
	if p.DefaultH264Encoder != "libx264" {
		t.Fatalf("encoder %q", p.DefaultH264Encoder)
	}
	if p.H264Summary == "" || !contains(p.H264Summary, "software") {
		t.Fatalf("summary %q", p.H264Summary)
	}
	if p.RecommendedMaxHeight != 720 {
		t.Fatalf("height %d", p.RecommendedMaxHeight)
	}
}

func TestPiModelFromDeviceTreeComputeModule4(t *testing.T) {
	model := "Raspberry Pi Compute Module 4 Rev 1.0"
	if piModelFromDeviceTreeModel(model) != Pi4 {
		t.Fatalf("expected Pi4 for %q", model)
	}
}

func TestPiEncodeProfilePi4V4L2(t *testing.T) {
	p := PiEncodeProfileForModel(Pi4)
	if p.DefaultH264Encoder != "h264_v4l2m2m" || !p.UseBitrateForV4L2M2M {
		t.Fatalf("%+v", p)
	}
}

func TestPiLegacySoftwareOnly(t *testing.T) {
	p := PiEncodeProfileForModel(PiLegacy)
	if p.DefaultH264Encoder != "libx264" || !contains(p.H264Summary, "software") {
		t.Fatalf("%+v", p)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || len(s) > 0 && stringIndex(s, sub) >= 0))
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

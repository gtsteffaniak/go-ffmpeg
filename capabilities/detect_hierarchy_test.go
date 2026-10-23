package capabilities

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestNonSoftwareChainDevices(t *testing.T) {
	chain := []platform.GPUChoice{
		{Enabled: true, Name: "NVIDIA GeForce RTX 3050"},
		{SoftwareOnly: true, Mode: platform.GPUModeSoftware},
	}
	if n := nonSoftwareChainDevices(chain); n != 1 {
		t.Fatalf("n=%d want 1 — the software fallback must not count as a device", n)
	}
}

func TestUsableHierarchyFiltersUnavailable(t *testing.T) {
	caps := NewCapabilities()
	caps.Encoders["h264_nvenc"] = EncoderCapability{Name: "h264_nvenc", Compiled: true, Available: true, Kind: "nvenc"}
	got := usableHierarchy([]AccelType{AccelNVENC, AccelD3D12, AccelVAAPI, AccelQSV}, caps)
	if len(got) != 1 || got[0] != AccelNVENC {
		t.Fatalf("hierarchy=%v want [nvenc]", got)
	}
}

func TestUsableHierarchyFallsBackToSoftware(t *testing.T) {
	caps := NewCapabilities()
	got := usableHierarchy([]AccelType{AccelNVENC, AccelVAAPI}, caps)
	if len(got) != 1 || got[0] != AccelNone {
		t.Fatalf("hierarchy=%v want [none]", got)
	}
}

package capabilities_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestEncoderKindInHierarchy(t *testing.T) {
	t.Parallel()
	h := []capabilities.AccelType{capabilities.AccelQSV, capabilities.AccelVAAPI}
	if !capabilities.EncoderKindInHierarchyForTest("qsv", h) {
		t.Fatal("expected qsv in hierarchy")
	}
	if capabilities.EncoderKindInHierarchyForTest("nvenc", h) {
		t.Fatal("expected nvenc excluded")
	}
}

func TestHierarchyForGPUIntel(t *testing.T) {
	t.Parallel()
	h := capabilities.HierarchyForGPU("intel", capabilities.PlatformInfo{})
	if len(h) < 2 || h[0] != capabilities.AccelVAAPI || h[1] != capabilities.AccelQSV {
		t.Fatalf("intel hierarchy = %v, want VAAPI→QSV", h)
	}
}

func TestScopedPlatformNVIDIA(t *testing.T) {
	t.Parallel()
	base := platform.Info{Intel: true, NVIDIA: true, AMD: false, Details: map[string]string{"gpu": "both"}}
	scoped := platform.ScopedPlatform(base, platform.GPUChoice{Enabled: true, Vendor: "nvidia", Name: "RTX"})
	if scoped.Intel {
		t.Fatal("expected Intel gate cleared for NVIDIA selection")
	}
	if !scoped.NVIDIA {
		t.Fatal("expected NVIDIA gate preserved")
	}
}

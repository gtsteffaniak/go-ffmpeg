package capabilities_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestHierarchyForPowerSaveHybridLinux(t *testing.T) {
	plat := capabilities.PlatformInfo{OS: "linux", Intel: true, NVIDIA: true, VAAPI: true}
	got := capabilities.HierarchyForPowerSave(plat)
	want := []capabilities.AccelType{capabilities.AccelVAAPI, capabilities.AccelQSV, capabilities.AccelNVENC, capabilities.AccelAMF}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestResolveEncoderHierarchyPreference(t *testing.T) {
	plat := capabilities.PlatformInfo{OS: "linux", Intel: true, NVIDIA: true}
	h := capabilities.ResolveEncoderHierarchy(nil, capabilities.EncoderPreferencePowerSave, platform.GPUChoice{}, plat)
	if h[0] != capabilities.AccelVAAPI || h[2] != capabilities.AccelNVENC {
		t.Fatalf("powersave order = %v", h)
	}
}

func TestResolveEncoderHierarchyExplicitWins(t *testing.T) {
	explicit := []capabilities.AccelType{capabilities.AccelNVENC}
	plat := capabilities.PlatformInfo{OS: "linux"}
	h := capabilities.ResolveEncoderHierarchy(explicit, capabilities.EncoderPreferencePowerSave, platform.GPUChoice{}, plat)
	if len(h) != 1 || h[0] != capabilities.AccelNVENC {
		t.Fatalf("explicit = %v", h)
	}
}

func TestParseEncoderPreferenceAliases(t *testing.T) {
	for _, s := range []string{"powersave", "igpu-first", "integrated"} {
		p, err := capabilities.ParseEncoderPreference(s)
		if err != nil || p != capabilities.EncoderPreferencePowerSave {
			t.Fatalf("%q -> %v err=%v", s, p, err)
		}
	}
}

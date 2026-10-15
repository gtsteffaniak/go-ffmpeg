package platform

import "testing"

func TestScopedPlatformAppleSetsVideoToolbox(t *testing.T) {
	base := Info{OS: "darwin", Arch: "arm64", VideoToolbox: true}
	scoped := ScopedPlatform(base, GPUChoice{Enabled: true, Vendor: "apple", Name: "VideoToolbox"})

	if !scoped.VideoToolbox {
		t.Fatal("apple scope should enable the VideoToolbox gate")
	}
	if scoped.NVIDIA || scoped.Intel || scoped.AMD || scoped.QSV || scoped.QSVRuntime || scoped.VAAPI || scoped.D3D12 {
		t.Fatalf("apple scope should clear other gates, got %+v", scoped)
	}
}

func TestScopedPlatformDisabledReturnsBase(t *testing.T) {
	base := Info{OS: "darwin", VideoToolbox: true}
	got := ScopedPlatform(base, GPUChoice{})
	if got.OS != base.OS || got.VideoToolbox != base.VideoToolbox {
		t.Fatalf("disabled choice should return base unchanged, got %+v", got)
	}
}

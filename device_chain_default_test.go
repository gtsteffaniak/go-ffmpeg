package ffmpeg_test

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func hybridAMDNVEnv() platform.GPUResolveEnv {
	return platform.GPUResolveEnv{
		GOOS: "windows",
		Devices: []platform.GPUDevice{
			{ID: "amd-igpu", Name: "AMD Radeon Graphics", Vendor: "amd", Integrated: true, DeviceClass: "igpu"},
			{ID: "nv-dgpu", Name: "NVIDIA GeForce RTX 3050 Laptop GPU", Vendor: "nvidia", Integrated: false, DeviceClass: "dgpu"},
		},
		Intel:  false,
		NVIDIA: true,
		AMD:    true,
	}
}

func hybridIntelNVEnv() platform.GPUResolveEnv {
	return platform.GPUResolveEnv{
		GOOS: "linux",
		Devices: []platform.GPUDevice{
			{RenderNode: "/dev/dri/renderD128", ID: "intel-igpu", Name: "Intel UHD Graphics", Vendor: "intel", Integrated: true, DeviceClass: "igpu"},
			{RenderNode: "/dev/dri/renderD129", ID: "nv-dgpu", Name: "NVIDIA GeForce RTX 3050", Vendor: "nvidia", Integrated: false, DeviceClass: "dgpu"},
		},
		Intel:  true,
		NVIDIA: true,
	}
}

// hybridWSLAMDNVEnv models a WSL2 guest whose adapters come from powershell.exe
// interop: no DRM render nodes, AMD iGPU + NVIDIA dGPU.
func hybridWSLAMDNVEnv() platform.GPUResolveEnv {
	return platform.GPUResolveEnv{
		GOOS: "linux",
		WSL:  true,
		Devices: []platform.GPUDevice{
			{ID: "win:PCI\\VEN_1002&DEV_164E", Name: "AMD Radeon(TM) Graphics", Vendor: "amd", Integrated: true, DeviceClass: "igpu"},
			{ID: "win:PCI\\VEN_10DE&DEV_25A2", Name: "NVIDIA GeForce RTX 3050 Laptop GPU", Vendor: "nvidia", Integrated: false, DeviceClass: "dgpu"},
		},
		NVIDIA: true,
		AMD:    true,
	}
}

func TestDefaultDeviceOrderWSLHybrid(t *testing.T) {
	chain, summary, err := ffmpeg.ResolveConfigDeviceChain(ffmpeg.Config{}, ffmpeg.GPUFallbackOnUnavailable, hybridWSLAMDNVEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 3 {
		t.Fatalf("chain %+v", chain)
	}
	if chain[0].Vendor != "amd" || chain[1].Vendor != "nvidia" || !chain[2].SoftwareOnly {
		t.Fatalf("%+v %+v %+v", chain[0], chain[1], chain[2])
	}
	if !strings.Contains(summary, "AMD Radeon") || !strings.Contains(summary, "RTX 3050") || !strings.Contains(summary, "software") {
		t.Fatalf("summary %q", summary)
	}
}

func TestZeroValueConfigUsesAutoDeviceHierarchy(t *testing.T) {
	var cfg ffmpeg.Config
	if !cfg.GPUSelector.IsZero() {
		t.Fatal("zero Config must leave GPUSelector at GPUAuto")
	}
	chain, summary, err := ffmpeg.ResolveConfigDeviceChain(cfg, ffmpeg.GPUFallbackOnUnavailable, hybridAMDNVEnv())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "Device order (auto)") {
		t.Fatalf("summary %q", summary)
	}
	if chain[0].Vendor != "amd" {
		t.Fatalf("%+v", chain[0])
	}
}

func TestParseGPUSelectorAutoMapsToGPUAuto(t *testing.T) {
	for _, in := range []string{"", "auto", "AUTO"} {
		sel, err := ffmpeg.ParseGPUSelector(in)
		if err != nil || sel.Kind != ffmpeg.GPUAuto || !sel.IsZero() {
			t.Fatalf("%q -> %+v err=%v", in, sel, err)
		}
	}
}

func TestDefaultDeviceOrderIntegratedFirst(t *testing.T) {
	chain, summary, err := ffmpeg.ResolveConfigDeviceChain(ffmpeg.Config{}, ffmpeg.GPUFallbackOnUnavailable, hybridAMDNVEnv())
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) < 3 {
		t.Fatalf("chain %+v", chain)
	}
	if !strings.Contains(summary, "Device order (auto)") || !strings.Contains(summary, "AMD Radeon") || !strings.Contains(summary, "RTX 3050") || !strings.Contains(summary, "software") {
		t.Fatalf("summary %q", summary)
	}
	if chain[0].Vendor != "amd" || chain[1].Vendor != "nvidia" || !chain[2].SoftwareOnly {
		t.Fatalf("%+v %+v %+v", chain[0], chain[1], chain[2])
	}
}

func TestDefaultDeviceOrderPreferDiscrete(t *testing.T) {
	cfg := ffmpeg.Config{DevicePreference: ffmpeg.PreferDiscrete}
	chain, summary, err := ffmpeg.ResolveConfigDeviceChain(cfg, ffmpeg.GPUFallbackOnUnavailable, hybridIntelNVEnv())
	if err != nil {
		t.Fatal(err)
	}
	if chain[0].Vendor != "nvidia" || chain[1].Vendor != "intel" {
		t.Fatalf("%+v %+v", chain[0], chain[1])
	}
	if !strings.Contains(summary, "prefer discrete") {
		t.Fatalf("summary %q", summary)
	}
}

func TestDefaultDeviceOrderSingleDGPU(t *testing.T) {
	env := platform.GPUResolveEnv{
		GOOS: "linux",
		Devices: []platform.GPUDevice{
			{ID: "nv", Name: "NVIDIA GeForce RTX 4090", Vendor: "nvidia", DeviceClass: "dgpu"},
		},
		NVIDIA: true,
	}
	chain, _, err := ffmpeg.ResolveConfigDeviceChain(ffmpeg.Config{}, ffmpeg.GPUFallbackOnUnavailable, env)
	if err != nil || len(chain) != 2 || chain[0].Vendor != "nvidia" {
		t.Fatalf("%+v err=%v", chain, err)
	}
}

func TestDefaultDeviceOrderNoGPUSoftwareOnly(t *testing.T) {
	env := platform.GPUResolveEnv{GOOS: "linux", Devices: nil}
	chain, summary, err := ffmpeg.ResolveConfigDeviceChain(ffmpeg.Config{}, ffmpeg.GPUFallbackOnUnavailable, env)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) < 1 || !chain[len(chain)-1].SoftwareOnly {
		t.Fatalf("%+v", chain)
	}
	if summary == "" {
		t.Fatal("expected summary")
	}
}

func TestEncoderPerformanceMapsToPreferDiscrete(t *testing.T) {
	cfg := ffmpeg.Config{EncoderPreference: "performance"}
	chain, _, err := ffmpeg.ResolveConfigDeviceChain(cfg, ffmpeg.GPUFallbackOnUnavailable, hybridIntelNVEnv())
	if err != nil {
		t.Fatal(err)
	}
	if chain[0].Vendor != "nvidia" {
		t.Fatalf("expected nvidia first, got %s", chain[0].Vendor)
	}
}

func TestExplicitDeviceSelectionOverridesPreference(t *testing.T) {
	cfg := ffmpeg.Config{
		DevicePreference: ffmpeg.PreferDiscrete,
		DeviceSelection: ffmpeg.DeviceSelection{
			Primary: ffmpeg.GPUByName("AMD Radeon"),
		},
	}
	chain, summary, err := ffmpeg.ResolveConfigDeviceChain(cfg, ffmpeg.GPUFallbackOnUnavailable, hybridAMDNVEnv())
	if err != nil {
		t.Fatal(err)
	}
	if chain[0].Vendor != "amd" {
		t.Fatalf("%+v", chain[0])
	}
	if !strings.Contains(summary, "explicit") {
		t.Fatalf("%q", summary)
	}
}

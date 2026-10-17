package platform_test

import (
	"errors"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func hybridLinuxEnv() platform.GPUResolveEnv {
	return platform.GPUResolveEnv{
		GOOS: "linux",
		Devices: []platform.GPUDevice{
			{RenderNode: "/dev/dri/renderD128", Vendor: "intel", Name: "Intel UHD Graphics", Integrated: true},
			{RenderNode: "/dev/dri/renderD129", Vendor: "nvidia", Name: "NVIDIA GeForce RTX 3050", Integrated: false},
		},
		Intel:  true,
		NVIDIA: true,
		Stat: func(path string) error {
			if path == "/dev/dri/renderD129" || path == "/dev/dri/renderD128" {
				return nil
			}
			return errors.New("not found")
		},
	}
}

func TestResolveGPURequestHybrid(t *testing.T) {
	env := hybridLinuxEnv()
	igpu := platform.GPURequest{Kind: platform.SelectionIntegrated, Label: "igpu"}
	c, err := platform.ResolveGPURequest(igpu, env, false)
	if err != nil || c.Vendor != "intel" || !c.Enabled {
		t.Fatalf("igpu: %+v err=%v", c, err)
	}
	dgpu := platform.GPURequest{Kind: platform.SelectionDiscrete, Label: "dgpu"}
	c, err = platform.ResolveGPURequest(dgpu, env, false)
	if err != nil || c.Vendor != "nvidia" {
		t.Fatalf("dgpu: %+v", c)
	}
}

func TestResolveGPURequestStrictIGPU(t *testing.T) {
	env := platform.GPUResolveEnv{GOOS: "linux", Devices: nil}
	_, err := platform.ResolveGPURequest(platform.GPURequest{Kind: platform.SelectionIntegrated, Label: "igpu"}, env, true)
	if err == nil {
		t.Fatal("expected strict error")
	}
}

func TestResolveGPURequestFallbackIGPU(t *testing.T) {
	env := platform.GPUResolveEnv{GOOS: "linux", Devices: nil}
	c, err := platform.ResolveGPURequest(platform.GPURequest{Kind: platform.SelectionIntegrated, Label: "igpu"}, env, false)
	if err != nil || !c.AutoSelect || !c.Fallback {
		t.Fatalf("got %+v", c)
	}
}

func TestResolveGPURequestWSLNVIDIAWithoutDRM(t *testing.T) {
	env := platform.GPUResolveEnv{
		GOOS:    "linux",
		WSL:     true,
		NVIDIA:  true,
		Devices: nil,
	}
	c, err := platform.ResolveGPURequest(platform.GPURequest{Kind: platform.SelectionDiscrete, Label: "dgpu"}, env, true)
	if err != nil || !c.Enabled || c.Vendor != "nvidia" || c.Mode != platform.GPUModeDGPU {
		t.Fatalf("dgpu: %+v err=%v", c, err)
	}
	auto, err := platform.ResolveGPURequest(platform.GPURequest{Kind: platform.SelectionAuto, Label: "auto"}, env, false)
	if err != nil || !auto.Enabled || auto.Vendor != "nvidia" || !auto.AutoSelect {
		t.Fatalf("auto: %+v err=%v", auto, err)
	}
}

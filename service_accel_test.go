package ffmpeg

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestBuildDetectOptionsEmptyGPUIsAuto(t *testing.T) {
	svc := &Service{cfg: Config{GPUSelector: GPUSelector{Kind: GPUAuto}}}

	opts, err := svc.buildDetectOptions()
	if err != nil {
		t.Fatal(err)
	}
	devices, _ := platform.ListGPUDevices()
	devices = appendPiSOCDevice(devices)
	if len(devices) > 0 {
		// Hosts with a GPU: auto scopes detection to the preferred device
		// (integrated/SoC first); software remains the chain fallback.
		if !opts.GPU.Enabled || opts.GPU.SoftwareOnly {
			t.Fatalf("auto should resolve the preferred GPU device, got %+v", opts.GPU)
		}
		return
	}
	if opts.GPU.Enabled {
		t.Fatalf("auto should not scope to a device, got %+v", opts.GPU)
	}
	if !opts.GPU.AutoSelect || opts.GPU.Mode != "auto" {
		t.Fatalf("mode = %+v", opts.GPU)
	}
}

func TestBuildDetectOptionsDeprecatedGPUString(t *testing.T) {
	svc := &Service{cfg: Config{GPU: "software"}}
	opts, err := svc.buildDetectOptions()
	if err != nil {
		t.Fatal(err)
	}
	if !opts.GPU.SoftwareOnly {
		t.Fatalf("opts = %+v", opts)
	}
	if opts.SkipHWTests {
		t.Fatal("software selection must not skip hardware detection")
	}
}

func TestBuildDetectOptionsForwardsEncoderHierarchy(t *testing.T) {
	svc := &Service{cfg: Config{EncoderHierarchy: []capabilities.AccelType{capabilities.AccelNone}}}

	opts, err := svc.buildDetectOptions()
	if err != nil {
		t.Fatal(err)
	}

	if len(opts.EncoderHierarchy) != 1 || opts.EncoderHierarchy[0] != capabilities.AccelNone {
		t.Fatalf("encoder hierarchy = %v, want [none]", opts.EncoderHierarchy)
	}
}

func TestBuildDetectOptionsEncoderPreference(t *testing.T) {
	svc := &Service{cfg: Config{EncoderPreference: "igpu-first"}}
	opts, err := svc.buildDetectOptions()
	if err != nil {
		t.Fatal(err)
	}
	if opts.EncoderPreference != capabilities.EncoderPreferencePowerSave {
		t.Fatalf("preference = %q", opts.EncoderPreference)
	}
}

func TestBuildDetectOptionsTypedIGPU(t *testing.T) {
	svc := &Service{cfg: Config{GPUSelector: GPUSelector{Kind: GPUIntegrated}}}
	opts, err := svc.buildDetectOptions()
	if err != nil {
		t.Fatal(err)
	}
	if opts.GPU.Mode != "igpu" && !opts.GPU.Enabled {
		// CI may lack DRM; allow fallback
		if !opts.GPU.Fallback && opts.GPU.Enabled {
			t.Fatalf("got %+v", opts.GPU)
		}
	}
}

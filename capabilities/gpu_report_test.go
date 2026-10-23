package capabilities_test

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestGPUForAccelHybridSimulation(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Platform = capabilities.PlatformInfo{
		OS:     "linux",
		Intel:  true,
		NVIDIA: true,
		VAAPI:  true,
	}

	// Simulate two GPUs without sysfs (ListGPUs may be empty in CI). On WSL the
	// host's adapters arrive via powershell interop, so the iGPU may be AMD —
	// derive the expectation from the actual integrated device.
	if gpus, err := platform.ListGPUs(); err == nil && len(gpus) >= 2 {
		nv := capabilities.GPUForAccel(caps, capabilities.AccelNVENC)
		va := capabilities.GPUForAccel(caps, capabilities.AccelVAAPI)
		if nv.Vendor != "nvidia" {
			t.Fatalf("nvenc vendor = %q", nv.Vendor)
		}
		wantVA := "intel"
		for _, g := range gpus {
			if g.Integrated {
				wantVA = g.Vendor
				break
			}
		}
		if va.Vendor != wantVA {
			t.Fatalf("vaapi vendor = %q, want %q", va.Vendor, wantVA)
		}
		if nv.RenderNode == va.RenderNode && nv.RenderNode != "" {
			t.Fatalf("expected different render nodes, both %q", nv.RenderNode)
		}
		return
	}

	path := capabilities.FormatAccelGPUPath(caps, capabilities.AccelNVENC)
	if !strings.Contains(path, "NVENC") {
		t.Fatalf("path = %q", path)
	}
}

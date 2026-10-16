package platform_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestDefaultRenderNodeForGPUsHybridPrefersIGPU(t *testing.T) {
	gpus := []platform.GPUDevice{
		{RenderNode: "/dev/dri/renderD128", Vendor: "nvidia", Integrated: false},
		{RenderNode: "/dev/dri/renderD129", Vendor: "intel", Integrated: true},
	}
	if got := platform.DefaultRenderNodeForGPUs(gpus); got != "/dev/dri/renderD129" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultRenderNodeForGPUsSingleGPU(t *testing.T) {
	gpus := []platform.GPUDevice{{RenderNode: "/dev/dri/renderD128", Integrated: false}}
	if got := platform.DefaultRenderNodeForGPUs(gpus); got != "/dev/dri/renderD128" {
		t.Fatalf("got %q", got)
	}
}

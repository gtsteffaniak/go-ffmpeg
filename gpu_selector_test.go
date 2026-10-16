package ffmpeg_test

import (
	"errors"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func hybridEnv() platform.GPUResolveEnv {
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

func TestParseGPUSelectorTable(t *testing.T) {
	cases := []struct {
		in   string
		kind ffmpeg.GPUKind
	}{
		{"", ffmpeg.GPUAuto},
		{"auto", ffmpeg.GPUAuto},
		{"IGPU", ffmpeg.GPUIntegrated},
		{"dgpu", ffmpeg.GPUDiscrete},
		{"cpu", ffmpeg.GPUSoftware},
		{"/dev/dri/renderD129", ffmpeg.GPUDevice},
		{"RTX 3050", ffmpeg.GPUDevice},
	}
	for _, tc := range cases {
		sel, err := ffmpeg.ParseGPUSelector(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if sel.Kind != tc.kind {
			t.Fatalf("%q kind=%v", tc.in, sel.Kind)
		}
	}
}

func TestResolveGPUSelectorHybrid(t *testing.T) {
	env := hybridEnv()
	c, err := ffmpeg.ResolveGPUSelectorWithEnv(ffmpeg.GPUSelector{Kind: ffmpeg.GPUIntegrated}, ffmpeg.GPUFallbackOnUnavailable, env)
	if err != nil || c.Vendor != "intel" {
		t.Fatalf("%+v err=%v", c, err)
	}
	c, err = ffmpeg.ResolveGPUSelectorWithEnv(ffmpeg.GPUByName("RTX 3050"), ffmpeg.GPUFallbackOnUnavailable, env)
	if err != nil || c.RenderDevice != "/dev/dri/renderD129" {
		t.Fatalf("%+v err=%v", c, err)
	}
}

func TestResolveGPUSelectorStrict(t *testing.T) {
	env := platform.GPUResolveEnv{GOOS: "linux"}
	_, err := ffmpeg.ResolveGPUSelectorWithEnv(ffmpeg.GPUSelector{Kind: ffmpeg.GPUDiscrete}, ffmpeg.GPUStrictOnUnavailable, env)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseGPUSelectorDeprecatedCompat(t *testing.T) {
	sel, err := ffmpeg.ParseGPUSelector("software")
	if err != nil || sel.Kind != ffmpeg.GPUSoftware {
		t.Fatalf("got %+v err=%v", sel, err)
	}
}

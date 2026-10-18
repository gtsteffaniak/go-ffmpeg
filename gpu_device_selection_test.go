package ffmpeg_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestParseGPUSelectorListOrdered(t *testing.T) {
	list, err := ffmpeg.ParseGPUSelectorList("RTX 3050, igpu")
	if err != nil || len(list) != 2 {
		t.Fatalf("%v %v", list, err)
	}
	if list[0].NameMatch != "RTX 3050" || list[1].Kind != ffmpeg.GPUIntegrated {
		t.Fatalf("%+v", list)
	}
}

func TestResolveDeviceSelectionChain(t *testing.T) {
	env := hybridEnv()
	primary, fallbacks, err := ffmpeg.ResolveDeviceSelectionWithEnv(ffmpeg.DeviceSelection{
		Primary:   ffmpeg.GPUByName("RTX 3050"),
		Fallbacks: []ffmpeg.GPUSelector{{Kind: ffmpeg.GPUIntegrated}},
	}, ffmpeg.GPUFallbackOnUnavailable, env)
	if err != nil || primary.Vendor != "nvidia" || len(fallbacks) != 1 || fallbacks[0].Vendor != "intel" {
		t.Fatalf("primary=%+v fallbacks=%+v err=%v", primary, fallbacks, err)
	}
}

func TestWindowsAdapterArgs(t *testing.T) {
	choice := platform.GPUChoice{Enabled: true, DeviceID: "win:abc", Name: "NVIDIA GPU", DXGIIndex: 1}
	args := platform.FFmpegHWDeviceInit("nvenc", choice, platform.Info{OS: "windows"})
	if len(args) != 2 || args[1] != "1" {
		t.Fatalf("%v", args)
	}
}

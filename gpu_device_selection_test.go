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

func TestWindowsNVENCUsesCUDAIndexFromPCI(t *testing.T) {
	choice := platform.GPUChoice{
		Enabled:    true,
		DeviceID:   "win:abc",
		Name:       "NVIDIA GPU",
		CUDAIndex:  0,
		PCIAddress: "00000000:01:00.0",
		Pinning:    platform.BackendPinning{NVENC: platform.PinningExact, CUDA: platform.PinningExact},
	}
	args := platform.FFmpegHWDeviceInit("nvenc", choice, platform.Info{OS: "windows"})
	if len(args) != 2 || args[0] != "-gpu" || args[1] != "0" {
		t.Fatalf("%v", args)
	}
}

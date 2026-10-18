package encode_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestV4L2M2MFallsBackAbove1080p(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.AllowSoftware = true
	caps.HardwarePlatform = capabilities.PlatformInfo{OS: "linux"}
	piProf := platform.PiEncodeProfileForModel(platform.Pi4)
	caps.PiProfile = &piProf
	caps.Encoders["h264_v4l2m2m"] = capabilities.EncoderCapability{Name: "h264_v4l2m2m", Compiled: true, Available: true, Kind: "v4l2m2m"}
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Compiled: true, Available: true, Kind: "software"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Preferred: capabilities.EncoderSelection{Encoder: "h264_v4l2m2m", Accel: capabilities.AccelV4L2M2M, Kind: "v4l2m2m", Fallback: "libx264"},
		Software:  []string{"libx264"},
	}
	r := encode.NewResolver(caps)
	sel, err := r.ResolveEncoder(encode.VideoProfile{
		Codec:        encode.CodecH264,
		OutputHeight: 1440,
		FrameRate:    30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sel.Encoder != "libx264" {
		t.Fatalf("expected libx264 fallback, got %s", sel.Encoder)
	}
}

func TestV4L2M2MUsedWithinLimits(t *testing.T) {
	caps := capabilities.NewCapabilities()
	piProf := platform.PiEncodeProfileForModel(platform.Pi4)
	caps.PiProfile = &piProf
	caps.Encoders["h264_v4l2m2m"] = capabilities.EncoderCapability{Name: "h264_v4l2m2m", Compiled: true, Available: true, Kind: "v4l2m2m"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Preferred: capabilities.EncoderSelection{Encoder: "h264_v4l2m2m", Accel: capabilities.AccelV4L2M2M},
	}
	r := encode.NewResolver(caps)
	sel, err := r.ResolveEncoder(encode.VideoProfile{Codec: encode.CodecH264, OutputHeight: 1080, FrameRate: 30})
	if err != nil || sel.Encoder != "h264_v4l2m2m" {
		t.Fatalf("%+v err=%v", sel, err)
	}
}

func TestResolutionFallbackDisallowedSoftware(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.AllowSoftware = false
	piProf := platform.PiEncodeProfileForModel(platform.Pi4)
	caps.PiProfile = &piProf
	caps.Encoders["h264_v4l2m2m"] = capabilities.EncoderCapability{Name: "h264_v4l2m2m", Compiled: true, Available: true, Kind: "v4l2m2m"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Preferred: capabilities.EncoderSelection{Encoder: "h264_v4l2m2m", Accel: capabilities.AccelV4L2M2M},
	}
	r := encode.NewResolver(caps)
	_, err := r.ResolveEncoder(encode.VideoProfile{Codec: encode.CodecH264, OutputHeight: 1440})
	if err == nil {
		t.Fatal("expected error when software fallback disabled")
	}
}

package encode_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestVideoFilterArgsVAAPIWithSoftwareDecode(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Platform.Details = map[string]string{"render_device": "/dev/dri/renderD128"}
	caps.Encoders["h264_vaapi"] = capabilities.EncoderCapability{Name: "h264_vaapi", Available: true, Kind: "vaapi"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Hardware:  map[capabilities.AccelType]string{capabilities.AccelVAAPI: "h264_vaapi"},
		Preferred: capabilities.EncoderSelection{Encoder: "h264_vaapi", Accel: capabilities.AccelVAAPI, Kind: "vaapi"},
	}
	r := encode.NewResolver(caps)
	args, err := r.VideoFilterArgs(
		encode.VideoProfile{Codec: encode.CodecH264, Accel: capabilities.AccelVAAPI},
		encode.VideoDecodeProfile{Codec: capabilities.CodecH264, ForceSoftware: true},
		1080,
	)
	if err != nil {
		t.Fatal(err)
	}
	s := joinArgs(args)
	if !containsAll(s, "-init_hw_device", "vaapi=va:/dev/dri/renderD128", "-filter_hw_device", "va", "hwupload") {
		t.Fatalf("args = %v", args)
	}
}

func TestVideoFilterArgsVideoToolboxSoftwareDecode(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["h264_videotoolbox"] = capabilities.EncoderCapability{Name: "h264_videotoolbox", Available: true, Kind: "videotoolbox"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Hardware:  map[capabilities.AccelType]string{capabilities.AccelVideoToolbox: "h264_videotoolbox"},
		Preferred: capabilities.EncoderSelection{Encoder: "h264_videotoolbox", Accel: capabilities.AccelVideoToolbox, Kind: "videotoolbox"},
	}
	r := encode.NewResolver(caps)
	args, err := r.VideoFilterArgs(
		encode.VideoProfile{Codec: encode.CodecH264},
		encode.VideoDecodeProfile{Codec: capabilities.CodecH264, ForceSoftware: true},
		720,
	)
	if err != nil {
		t.Fatal(err)
	}
	s := joinArgs(args)
	if !containsAll(s, "scale=-2:min(720\\,ih)", "format=nv12", "hwupload", "-filter_hw_device", "vt") {
		t.Fatalf("args = %v", args)
	}
	if contains(s, "-init_hw_device") {
		t.Fatalf("filter args must not include init_hw_device: %v", args)
	}
}

func TestVideoFilterArgsVideoToolbox(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Filters["scale_vt"] = true
	caps.Encoders["h264_videotoolbox"] = capabilities.EncoderCapability{Name: "h264_videotoolbox", Available: true, Kind: "videotoolbox"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Hardware:  map[capabilities.AccelType]string{capabilities.AccelVideoToolbox: "h264_videotoolbox"},
		Preferred: capabilities.EncoderSelection{Encoder: "h264_videotoolbox", Accel: capabilities.AccelVideoToolbox, Kind: "videotoolbox"},
		DecodePreferred: capabilities.DecoderSelection{
			Decoder: "hwaccel:videotoolbox:h264",
			Accel:   capabilities.AccelVideoToolbox,
			SWCodec: "h264",
		},
	}
	r := encode.NewResolver(caps)
	args, err := r.VideoFilterArgs(
		encode.VideoProfile{Codec: encode.CodecH264},
		encode.VideoDecodeProfile{Codec: capabilities.CodecH264},
		720,
	)
	if err != nil {
		t.Fatal(err)
	}
	s := joinArgs(args)
	if !contains(s, "scale_vt") {
		t.Fatalf("args = %v", args)
	}
}

func joinArgs(args []string) string {
	out := ""
	for _, a := range args {
		out += a + " "
	}
	return out
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}

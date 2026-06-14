package encode_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestVideoEncoderArgsH264Software(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Available: true, Kind: "software"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Software:  []string{"libx264"},
		Preferred: capabilities.EncoderSelection{Encoder: "libx264", Accel: capabilities.AccelNone, Kind: "software"},
	}
	r := encode.NewResolver(caps)
	args, err := r.VideoEncoderArgs(encode.VideoProfile{
		Codec:   encode.CodecH264,
		Bitrate: encode.BitrateConfig{Target: "4M", Max: "4M", BufSize: "8M", Min: "2M"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if args[1] != "libx264" {
		t.Fatalf("args = %v", args)
	}
}

func TestVideoEncoderArgsCopy(t *testing.T) {
	r := encode.NewResolver(capabilities.NewCapabilities())
	args, err := r.VideoEncoderArgs(encode.VideoProfile{Codec: encode.CodecCopy})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[1] != "copy" {
		t.Fatalf("args = %v", args)
	}
}

func TestQualityToQScale(t *testing.T) {
	if encode.QualityToQScale(0) != "1" {
		t.Fatal("expected clamp to 1")
	}
	if encode.QualityToQScale(5) != "5" {
		t.Fatal("expected 5")
	}
}

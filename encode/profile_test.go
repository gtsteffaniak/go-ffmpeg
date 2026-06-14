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

func TestVideoEncoderArgsH264QSV(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["h264_qsv"] = capabilities.EncoderCapability{Name: "h264_qsv", Available: true, Kind: "qsv"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Hardware:  map[capabilities.AccelType]string{capabilities.AccelQSV: "h264_qsv"},
		Preferred: capabilities.EncoderSelection{Encoder: "h264_qsv", Accel: capabilities.AccelQSV, Kind: "qsv"},
	}
	r := encode.NewResolver(caps)
	args, err := r.VideoEncoderArgs(encode.VideoProfile{
		Codec:   encode.CodecH264,
		Quality: encode.PresetVeryfast,
		Bitrate: encode.BitrateConfig{Target: "4M", Max: "4M", BufSize: "8M"},
		GOP:     60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if args[1] != "h264_qsv" {
		t.Fatalf("encoder = %v", args)
	}
	presetIdx := indexOf(args, "-preset")
	if presetIdx < 0 || args[presetIdx+1] != "7" {
		t.Fatalf("expected numeric QSV preset 7, got %v", args)
	}
	if pixIdx := indexOf(args, "-pix_fmt"); pixIdx < 0 || args[pixIdx+1] != "nv12" {
		t.Fatalf("expected nv12 pixel format for QSV, got %v", args)
	}
	for _, forbidden := range []string{"-load_plugin", "ultrafast"} {
		if indexOf(args, forbidden) >= 0 {
			t.Fatalf("unexpected QSV arg %q in %v", forbidden, args)
		}
	}
}

func TestVideoEncoderArgsQualityAMF(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["h264_amf"] = capabilities.EncoderCapability{Name: "h264_amf", Available: true, Kind: "amf"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Hardware:  map[capabilities.AccelType]string{capabilities.AccelAMF: "h264_amf"},
		Preferred: capabilities.EncoderSelection{Encoder: "h264_amf", Accel: capabilities.AccelAMF, Kind: "amf"},
	}
	r := encode.NewResolver(caps)

	fast, err := r.VideoEncoderArgs(encode.VideoProfile{
		Codec: encode.CodecH264, Quality: encode.PresetUltrafast,
		Bitrate: encode.BitrateConfig{Target: "2M", Max: "2M", BufSize: "4M"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if idx := indexOf(fast, "-quality_preset"); idx < 0 || fast[idx+1] != "speed" {
		t.Fatalf("ultrafast AMF h264: got %v", fast)
	}

	quality, err := r.VideoEncoderArgs(encode.VideoProfile{
		Codec: encode.CodecH264, Quality: encode.PresetMedium,
		Bitrate: encode.BitrateConfig{Target: "2M", Max: "2M", BufSize: "4M"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if idx := indexOf(quality, "-quality_preset"); idx < 0 || quality[idx+1] != "quality" {
		t.Fatalf("medium AMF h264: got %v", quality)
	}
}

func TestVideoEncoderArgsQualityUsesProfileGOP(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Available: true, Kind: "software"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Software:  []string{"libx264"},
		Preferred: capabilities.EncoderSelection{Encoder: "libx264", Accel: capabilities.AccelNone, Kind: "software"},
	}
	r := encode.NewResolver(caps)
	args, err := r.VideoEncoderArgs(encode.VideoProfile{
		Codec:         encode.CodecH264,
		Quality:       encode.PresetFast,
		ForceSoftware: true,
		GOP:           120,
		Bitrate:       encode.BitrateConfig{Target: "2M", Max: "2M", BufSize: "4M"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if idx := indexOf(args, "-g"); idx < 0 || args[idx+1] != "120" {
		t.Fatalf("expected gop 120, got %v", args)
	}
	if idx := indexOf(args, "-preset"); idx < 0 || args[idx+1] != "fast" {
		t.Fatalf("expected x264 preset fast, got %v", args)
	}
}

func indexOf(slice []string, val string) int {
	for i, s := range slice {
		if s == val {
			return i
		}
	}
	return -1
}

func TestQualityToQScale(t *testing.T) {
	if encode.QualityToQScale(0) != "1" {
		t.Fatal("expected clamp to 1")
	}
	if encode.QualityToQScale(5) != "5" {
		t.Fatal("expected 5")
	}
}

package capabilities_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestBuildEncodeOptions(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Available: true, Kind: "software"}
	caps.Encoders["h264_qsv"] = capabilities.EncoderCapability{Name: "h264_qsv", Available: true, Kind: "qsv"}
	caps.Encoders["h264_nvenc"] = capabilities.EncoderCapability{Name: "h264_nvenc", Available: false, TestError: "[platform] no NVIDIA GPU"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Software:  []string{"libx264"},
		Hardware:  map[capabilities.AccelType]string{capabilities.AccelQSV: "h264_qsv"},
		Preferred: capabilities.EncoderSelection{Encoder: "h264_qsv", Accel: capabilities.AccelQSV, Kind: "qsv"},
	}
	capabilities.BuildEncodeOptions(caps)

	if len(caps.EncodeOptions) == 0 {
		t.Fatal("expected encode options")
	}
	available := caps.AvailableEncodeOptions()
	if len(available) != 2 {
		t.Fatalf("available = %d, want 2", len(available))
	}
	forCodec := caps.EncodeOptionsForCodec(capabilities.CodecH264)
	if len(forCodec) < 2 {
		t.Fatalf("h264 options = %d", len(forCodec))
	}
	var defaultCount int
	for _, opt := range forCodec {
		if opt.Default {
			defaultCount++
		}
		if opt.Encoder == "h264_nvenc" && opt.Available {
			t.Fatal("nvenc should be unavailable")
		}
	}
	if defaultCount != 1 {
		t.Fatalf("default count = %d", defaultCount)
	}
}

func TestEncoderMatchesCodec(t *testing.T) {
	if !capabilities.EncoderMatchesCodec("libx264", capabilities.CodecH264) {
		t.Fatal("libx264 should match h264")
	}
	if capabilities.EncoderMatchesCodec("libx264", capabilities.CodecAV1) {
		t.Fatal("libx264 should not match av1")
	}
}

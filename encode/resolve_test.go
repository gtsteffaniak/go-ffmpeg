package encode_test

import (
	"errors"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func testCaps() *capabilities.Capabilities {
	caps := capabilities.NewCapabilities()
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Available: true, Kind: "software"}
	caps.Encoders["h264_qsv"] = capabilities.EncoderCapability{Name: "h264_qsv", Available: true, Kind: "qsv"}
	caps.Encoders["libsvtav1"] = capabilities.EncoderCapability{Name: "libsvtav1", Available: true, Kind: "software"}
	caps.Encoders["librav1e"] = capabilities.EncoderCapability{Name: "librav1e", Available: false, TestError: "not installed"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Software:  []string{"libx264"},
		Hardware:  map[capabilities.AccelType]string{capabilities.AccelQSV: "h264_qsv"},
		Preferred: capabilities.EncoderSelection{Encoder: "h264_qsv", Accel: capabilities.AccelQSV, Kind: "qsv", Fallback: "libx264"},
	}
	caps.CodecMatrix[capabilities.CodecAV1] = capabilities.CodecSupport{
		Software:  []string{"libsvtav1", "librav1e"},
		Preferred: capabilities.EncoderSelection{Encoder: "libsvtav1", Accel: capabilities.AccelNone, Kind: "software"},
	}
	capabilities.BuildEncodeOptions(caps)
	return caps
}

func TestResolveEncoderDefaultPreferred(t *testing.T) {
	r := encode.NewResolver(testCaps())
	sel, err := r.ResolveEncoder(encode.VideoProfile{Codec: encode.CodecH264})
	if err != nil {
		t.Fatal(err)
	}
	if sel.Encoder != "h264_qsv" || sel.Accel != capabilities.AccelQSV {
		t.Fatalf("sel = %+v", sel)
	}
}

func TestResolveEncoderForceSoftware(t *testing.T) {
	r := encode.NewResolver(testCaps())
	sel, err := r.ResolveEncoder(encode.VideoProfile{Codec: encode.CodecH264, ForceSoftware: true})
	if err != nil {
		t.Fatal(err)
	}
	if sel.Encoder != "libx264" || sel.Accel != capabilities.AccelNone {
		t.Fatalf("sel = %+v", sel)
	}
}

func TestResolveEncoderAccelOverride(t *testing.T) {
	r := encode.NewResolver(testCaps())
	sel, err := r.ResolveEncoder(encode.VideoProfile{Codec: encode.CodecH264, Accel: capabilities.AccelQSV})
	if err != nil {
		t.Fatal(err)
	}
	if sel.Encoder != "h264_qsv" {
		t.Fatalf("sel = %+v", sel)
	}
}

func TestResolveEncoderExplicitSoftware(t *testing.T) {
	r := encode.NewResolver(testCaps())
	sel, err := r.ResolveEncoder(encode.VideoProfile{Codec: encode.CodecAV1, Encoder: "libsvtav1"})
	if err != nil {
		t.Fatal(err)
	}
	if sel.Encoder != "libsvtav1" {
		t.Fatalf("sel = %+v", sel)
	}
}

func TestResolveEncoderUnavailable(t *testing.T) {
	r := encode.NewResolver(testCaps())
	_, err := r.ResolveEncoder(encode.VideoProfile{Codec: encode.CodecAV1, Encoder: "librav1e"})
	if err == nil {
		t.Fatal("expected error")
	}
	var pe *encode.ProfileError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProfileError, got %T: %v", err, err)
	}
	if !errors.Is(err, encode.ErrProfileUnsupported) {
		t.Fatalf("expected ErrProfileUnsupported, got %v", err)
	}
}

func TestResolveEncoderConflict(t *testing.T) {
	r := encode.NewResolver(testCaps())
	_, err := r.ResolveEncoder(encode.VideoProfile{
		Codec:         encode.CodecH264,
		Encoder:       "h264_qsv",
		ForceSoftware: true,
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestResolveEncoderBadAccelForCodec(t *testing.T) {
	r := encode.NewResolver(testCaps())
	_, err := r.ResolveEncoder(encode.VideoProfile{Codec: encode.CodecH264, Accel: capabilities.AccelAMF})
	if err == nil {
		t.Fatal("expected error for AMF on h264 without amf encoder")
	}
}

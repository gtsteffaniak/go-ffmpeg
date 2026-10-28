package encode_test

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func hwFilterCaps(encoder string, accel capabilities.AccelType, kind string, dec capabilities.DecoderSelection) *capabilities.Capabilities {
	caps := capabilities.NewCapabilities()
	caps.Platform.Details = map[string]string{"render_device": "/dev/dri/renderD128"}
	caps.Encoders[encoder] = capabilities.EncoderCapability{Name: encoder, Kind: kind, Compiled: true, Available: true}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Hardware:        map[capabilities.AccelType]string{accel: encoder},
		Preferred:       capabilities.EncoderSelection{Encoder: encoder, Accel: accel, Kind: kind},
		DecodePreferred: dec,
	}
	caps.CodecMatrix[capabilities.CodecAV1] = capabilities.CodecSupport{DecodePreferred: dec}
	return caps
}

func vaapiDecode() capabilities.DecoderSelection {
	return capabilities.DecoderSelection{Decoder: "hwaccel:vaapi:av1", Accel: capabilities.AccelVAAPI, Kind: "vaapi", SWCodec: "av1"}
}

func filterString(t *testing.T, caps *capabilities.Capabilities, decode encode.VideoDecodeProfile) string {
	t.Helper()
	args, err := encode.NewResolver(caps).VideoFilterArgs(encode.VideoProfile{Codec: encode.CodecH264}, decode, 720)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(args, " ")
}

// VAAPI surfaces must never reach a software scale filter ("Impossible to
// convert between the formats ... src: vaapi").
func TestVAAPIHwDecodeUsesScaleVAAPI(t *testing.T) {
	caps := hwFilterCaps("h264_vaapi", capabilities.AccelVAAPI, "vaapi", vaapiDecode())
	caps.Decoders["hwaccel:vaapi:av1"] = capabilities.DecoderCapability{Name: "hwaccel:vaapi:av1", Available: true, Compiled: true}
	caps.Filters["scale_vaapi"] = true

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1})
	if !strings.Contains(got, "scale_vaapi=w=-2:h=min(720\\,ih):format=nv12") {
		t.Fatalf("want scale_vaapi graph, got %q", got)
	}
	if strings.Contains(got, "hwupload") || strings.Contains(got, "-init_hw_device") {
		t.Fatalf("hw decode must not upload or re-init the device: %q", got)
	}
}

func TestVAAPIHwDecodeWithoutScaleVAAPIDownloads(t *testing.T) {
	caps := hwFilterCaps("h264_vaapi", capabilities.AccelVAAPI, "vaapi", vaapiDecode())
	caps.Decoders["hwaccel:vaapi:av1"] = capabilities.DecoderCapability{Name: "hwaccel:vaapi:av1", Available: true, Compiled: true}

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1})
	if !strings.Contains(got, "hwdownload,format=nv12|p010le,scale=-2:min(720\\,ih),format=nv12,hwupload") {
		t.Fatalf("want hwdownload fallback graph, got %q", got)
	}
}

func TestVAAPISoftwareDecodeStillUploads(t *testing.T) {
	caps := hwFilterCaps("h264_vaapi", capabilities.AccelVAAPI, "vaapi", vaapiDecode())
	caps.Filters["scale_vaapi"] = true

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1, ForceSoftware: true})
	if !strings.Contains(got, "scale=-2:min(720\\,ih),format=nv12,hwupload") || !strings.Contains(got, "-init_hw_device vaapi=va:") {
		t.Fatalf("software decode must scale in software and upload, got %q", got)
	}
	if strings.Contains(got, "scale_vaapi") || strings.Contains(got, "hwdownload") {
		t.Fatalf("unexpected hw filter on software decode: %q", got)
	}
}

func cudaDecode() capabilities.DecoderSelection {
	return capabilities.DecoderSelection{Decoder: "av1_cuvid", Accel: capabilities.AccelNVENC, Kind: "nvenc"}
}

// CUDA surfaces must never reach a software scale filter ("Impossible to
// convert between the formats ... src: cuda").
func TestNVENCHwDecodeUsesScaleCuda(t *testing.T) {
	caps := hwFilterCaps("h264_nvenc", capabilities.AccelNVENC, "nvenc", cudaDecode())
	caps.Decoders["av1_cuvid"] = capabilities.DecoderCapability{Name: "av1_cuvid", Available: true, Compiled: true}
	caps.Filters["scale_cuda"] = true

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1})
	if !strings.Contains(got, "-vf scale_cuda=w=-2:h=min(720\\,ih):format=nv12,hwdownload,format=nv12") {
		t.Fatalf("want scale_cuda graph, got %q", got)
	}
}

// Without scale_cuda the decoder emits system-memory frames (see decode args),
// so the plain software graph (which also converts 10-bit) applies.
func TestNVENCHwDecodeWithoutScaleCudaUsesSoftwareGraph(t *testing.T) {
	caps := hwFilterCaps("h264_nvenc", capabilities.AccelNVENC, "nvenc", cudaDecode())
	caps.Decoders["av1_cuvid"] = capabilities.DecoderCapability{Name: "av1_cuvid", Available: true, Compiled: true}

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1})
	if strings.Contains(got, "hwdownload") || !strings.Contains(got, "scale=-2:min(720\\,ih),format=yuv420p") {
		t.Fatalf("want software graph, got %q", got)
	}
}

func TestNVENCSoftwareDecodeKeepsSoftwareScale(t *testing.T) {
	caps := hwFilterCaps("h264_nvenc", capabilities.AccelNVENC, "nvenc", cudaDecode())
	caps.Filters["scale_cuda"] = true

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1, ForceSoftware: true})
	if !strings.Contains(got, "scale=-2:min(720\\,ih),format=yuv420p") || strings.Contains(got, "cuda") {
		t.Fatalf("software decode graph wrong: %q", got)
	}
}

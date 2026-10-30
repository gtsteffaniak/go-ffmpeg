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

// softwareEncFilterCaps builds caps whose preferred encoder resolves to software
// (libx264) while decode uses the supplied hardware decoder selection.
func softwareEncFilterCaps(dec capabilities.DecoderSelection) *capabilities.Capabilities {
	caps := capabilities.NewCapabilities()
	caps.Platform.Details = map[string]string{"render_device": "/dev/dri/renderD128"}
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Kind: "software", Compiled: true, Available: true}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Preferred:       capabilities.EncoderSelection{Encoder: "libx264", Accel: capabilities.AccelNone, Kind: "software"},
		DecodePreferred: dec,
	}
	caps.CodecMatrix[capabilities.CodecAV1] = capabilities.CodecSupport{DecodePreferred: dec}
	return caps
}

func vaapiCudaStyleDecode() capabilities.DecoderSelection {
	return capabilities.DecoderSelection{Decoder: "hwaccel:vaapi:av1", Accel: capabilities.AccelVAAPI, Kind: "vaapi", SWCodec: "av1"}
}

// A software encoder (libx264) with CUDA decode must download the CUDA surfaces
// before any software filter: "Impossible to convert between the formats ...
// src: cuda" otherwise.
func TestSoftwareEncoderWithCUDADecodeDownloads(t *testing.T) {
	caps := softwareEncFilterCaps(cudaDecode())
	caps.Decoders["av1_cuvid"] = capabilities.DecoderCapability{Name: "av1_cuvid", Available: true, Compiled: true}
	caps.Filters["scale_cuda"] = true

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1})
	if !strings.Contains(got, "-vf hwdownload,format=nv12|p010le,scale=-2:min(720\\,ih),format=yuv420p") {
		t.Fatalf("want hwdownload software graph, got %q", got)
	}
	if strings.Contains(got, "cuda") {
		t.Fatalf("software encoder graph must not reference cuda: %q", got)
	}
}

// ForceSoftware on an otherwise-hardware profile must also download CUDA frames
// before the software graph.
func TestSoftwareEncoderForceSoftwareWithCUDADecodeDownloads(t *testing.T) {
	caps := softwareEncFilterCaps(cudaDecode())
	caps.Decoders["av1_cuvid"] = capabilities.DecoderCapability{Name: "av1_cuvid", Available: true, Compiled: true}
	caps.Filters["scale_cuda"] = true

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1, ForceSoftware: false})
	if !strings.Contains(got, "hwdownload") {
		t.Fatalf("want hwdownload first, got %q", got)
	}

	// With software decode there is nothing to download.
	soft := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1, ForceSoftware: true})
	if strings.Contains(soft, "hwdownload") {
		t.Fatalf("software decode must not hwdownload: %q", soft)
	}
}

// A software encoder with CUDA decode but no scale_cuda: the decoder emits
// system-memory frames, so the plain software graph applies.
func TestSoftwareEncoderWithCUDADecodeWithoutScaleCudaUsesSoftwareGraph(t *testing.T) {
	caps := softwareEncFilterCaps(cudaDecode())
	caps.Decoders["av1_cuvid"] = capabilities.DecoderCapability{Name: "av1_cuvid", Available: true, Compiled: true}

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1})
	if strings.Contains(got, "hwdownload") || !strings.Contains(got, "scale=-2:min(720\\,ih),format=yuv420p") {
		t.Fatalf("want plain software graph, got %q", got)
	}
}

// A software encoder with VAAPI decode must download the VAAPI surfaces first.
func TestSoftwareEncoderWithVAAPIDecodeDownloads(t *testing.T) {
	caps := softwareEncFilterCaps(vaapiCudaStyleDecode())
	caps.Decoders["hwaccel:vaapi:av1"] = capabilities.DecoderCapability{Name: "hwaccel:vaapi:av1", Available: true, Compiled: true}

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1})
	if !strings.Contains(got, "-vf hwdownload,format=nv12|p010le") {
		t.Fatalf("want vaapi hwdownload graph, got %q", got)
	}
	if strings.Contains(got, "scale_vaapi") || strings.Contains(got, "hwupload") {
		t.Fatalf("software encoder must not keep vaapi surfaces: %q", got)
	}
}

// NVENC encode with VAAPI decode must download the VAAPI surfaces (the NVENC
// branch only handles NVENC decode; anything else falls back to software).
func TestNVENCEncoderWithVAAPIDecodeDownloads(t *testing.T) {
	caps := hwFilterCaps("h264_nvenc", capabilities.AccelNVENC, "nvenc", vaapiCudaStyleDecode())
	caps.Decoders["hwaccel:vaapi:av1"] = capabilities.DecoderCapability{Name: "hwaccel:vaapi:av1", Available: true, Compiled: true}

	got := filterString(t, caps, encode.VideoDecodeProfile{Codec: capabilities.CodecAV1})
	if !strings.Contains(got, "hwdownload") {
		t.Fatalf("want hwdownload for vaapi decode under nvenc encode, got %q", got)
	}
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

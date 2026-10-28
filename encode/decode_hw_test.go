package encode_test

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestVideoDecoderArgsVAAPIOutputFormat(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Platform.Details = map[string]string{"render_device": "/dev/dri/renderD128"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		DecodePreferred: capabilities.DecoderSelection{
			Decoder: "hwaccel:vaapi:h264",
			Accel:   capabilities.AccelVAAPI,
			Kind:    "vaapi",
			SWCodec: "h264",
		},
	}
	r := encode.NewResolver(caps)
	args, err := r.VideoDecoderArgs(encode.VideoDecodeProfile{Codec: capabilities.CodecH264})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-hwaccel_output_format", "vaapi", "-hwaccel", "vaapi"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, args)
		}
	}
}

func TestVideoDecoderArgsCUDA(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		DecodePreferred: capabilities.DecoderSelection{
			Decoder: "h264_cuvid",
			Accel:   capabilities.AccelNVENC,
			Kind:    "nvenc",
		},
	}
	caps.Decoders["h264_cuvid"] = capabilities.DecoderCapability{Name: "h264_cuvid", Available: true, Compiled: true}
	caps.Filters["scale_cuda"] = true
	r := encode.NewResolver(caps)
	args, err := r.VideoDecoderArgs(encode.VideoDecodeProfile{Codec: capabilities.CodecH264})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda", "h264_cuvid"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, args)
		}
	}
}

func TestVideoDecoderArgsVulkanOutputFormat(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		DecodePreferred: capabilities.DecoderSelection{
			Decoder: "hwaccel:vulkan:h264",
			Accel:   capabilities.AccelVulkan,
			Kind:    "vulkan",
			SWCodec: "h264",
		},
	}
	r := encode.NewResolver(caps)
	args, err := r.VideoDecoderArgs(encode.VideoDecodeProfile{Codec: capabilities.CodecH264})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-hwaccel_output_format vulkan") {
		t.Fatalf("expected vulkan hw surface: %v", args)
	}
}

// Without scale_cuda nothing can consume CUDA surfaces, so frames must be
// downloaded to system memory by the decoder (no -hwaccel_output_format cuda).
func TestVideoDecoderArgsCUDAWithoutScaleCudaUsesSystemMemory(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		DecodePreferred: capabilities.DecoderSelection{Decoder: "h264_cuvid", Accel: capabilities.AccelNVENC, Kind: "nvenc"},
	}
	caps.Decoders["h264_cuvid"] = capabilities.DecoderCapability{Name: "h264_cuvid", Available: true, Compiled: true}
	args, err := encode.NewResolver(caps).VideoDecoderArgs(encode.VideoDecodeProfile{Codec: capabilities.CodecH264})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(args, " "), "-hwaccel_output_format") {
		t.Fatalf("must not keep CUDA surfaces without scale_cuda: %v", args)
	}
}

package encode_test

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestVulkanVideoFiltersInitAndUpload(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Platform = platform.Info{OS: "linux", Details: map[string]string{"render_device": "/dev/dri/renderD128"}}
	caps.Filters["scale_vulkan"] = true
	caps.Encoders["h264_vulkan"] = capabilities.EncoderCapability{Name: "h264_vulkan", Kind: "vulkan", Compiled: true, Available: true}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Hardware: map[capabilities.AccelType]string{capabilities.AccelVulkan: "h264_vulkan"},
		Preferred: capabilities.EncoderSelection{
			Encoder: "h264_vulkan",
			Accel:   capabilities.AccelVulkan,
			Kind:    "vulkan",
		},
	}
	r := encode.NewResolver(caps)
	args, err := r.VideoFilterArgs(encode.VideoProfile{Codec: encode.CodecH264}, encode.VideoDecodeProfile{}, 720)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "vulkan=vk:") || !strings.Contains(joined, "hwupload") {
		t.Fatalf("args=%v", args)
	}
}

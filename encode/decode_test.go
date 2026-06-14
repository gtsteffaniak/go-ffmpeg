package encode_test

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestVideoDecoderArgsVAAPI(t *testing.T) {
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
	if !strings.Contains(joined, "hwaccel") || !strings.Contains(joined, "h264") {
		t.Fatalf("args = %v", args)
	}
}

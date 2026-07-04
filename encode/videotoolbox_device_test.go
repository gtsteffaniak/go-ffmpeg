package encode_test

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestVideoToolboxPreInputInitSoftwareDecodeOnly(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["h264_videotoolbox"] = capabilities.EncoderCapability{Name: "h264_videotoolbox", Available: true, Kind: "videotoolbox"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Hardware:  map[capabilities.AccelType]string{capabilities.AccelVideoToolbox: "h264_videotoolbox"},
		Preferred: capabilities.EncoderSelection{Encoder: "h264_videotoolbox", Accel: capabilities.AccelVideoToolbox, Kind: "videotoolbox"},
	}
	r := encode.NewResolver(caps)
	args := r.VideoToolboxPreInputInit(
		encode.VideoDecodeProfile{Codec: capabilities.CodecH264, ForceSoftware: true},
		encode.VideoProfile{Codec: encode.CodecH264},
	)
	if len(args) != 2 || args[0] != "-init_hw_device" || args[1] != "videotoolbox=vt" {
		t.Fatalf("got %v", args)
	}
}

func TestVideoToolboxPreInputInitSkipsWhenHWDecode(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["h264_videotoolbox"] = capabilities.EncoderCapability{Name: "h264_videotoolbox", Available: true, Kind: "videotoolbox"}
	caps.Decoders["hwaccel:videotoolbox:h264"] = capabilities.DecoderCapability{Name: "hwaccel:videotoolbox:h264", Available: true, SWCodec: "h264"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Hardware:  map[capabilities.AccelType]string{capabilities.AccelVideoToolbox: "h264_videotoolbox"},
		Preferred: capabilities.EncoderSelection{Encoder: "h264_videotoolbox", Accel: capabilities.AccelVideoToolbox, Kind: "videotoolbox"},
		HardwareDecode: map[capabilities.AccelType]string{capabilities.AccelVideoToolbox: "hwaccel:videotoolbox:h264"},
		DecodePreferred: capabilities.DecoderSelection{
			Decoder: "hwaccel:videotoolbox:h264",
			Accel:   capabilities.AccelVideoToolbox,
			SWCodec: "h264",
		},
	}
	r := encode.NewResolver(caps)
	args := r.VideoToolboxPreInputInit(
		encode.VideoDecodeProfile{Codec: capabilities.CodecH264},
		encode.VideoProfile{Codec: encode.CodecH264},
	)
	if len(args) != 0 {
		t.Fatalf("expected no pre-input init when VT decode active, got %v", args)
	}
	decodeArgs, err := r.VideoDecoderArgs(encode.VideoDecodeProfile{Codec: capabilities.CodecH264})
	if err != nil {
		t.Fatal(err)
	}
	filterArgs, err := r.VideoFilterArgs(
		encode.VideoProfile{Codec: encode.CodecH264},
		encode.VideoDecodeProfile{Codec: capabilities.CodecH264},
		720,
	)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(append(decodeArgs, filterArgs...), " ")
	if strings.Count(joined, "-init_hw_device") != 1 {
		t.Fatalf("expected exactly one init_hw_device across decode+filter, got: %s", joined)
	}
	if strings.Contains(joined, "init_hw_device") && strings.Count(joined, "videotoolbox=vt") > 2 {
		t.Fatalf("unexpected duplicate vt device refs: %s", joined)
	}
}

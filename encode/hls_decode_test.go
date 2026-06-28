package encode_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestOnDemandHLSDecodeProfile(t *testing.T) {
	base := encode.VideoDecodeProfile{Codec: capabilities.CodecH264, Accel: capabilities.AccelQSV}
	out := encode.OnDemandHLSDecodeProfile(base)
	if !out.ForceSoftware || out.Accel != "" || out.Decoder != "" {
		t.Fatalf("got %+v", out)
	}
	already := encode.VideoDecodeProfile{ForceSoftware: true, Codec: capabilities.CodecH264}
	if encode.OnDemandHLSDecodeProfile(already) != already {
		t.Fatal("expected unchanged when already ForceSoftware")
	}
}

package encode_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestSoftwareDecodeProfile(t *testing.T) {
	t.Parallel()
	base := encode.VideoDecodeProfile{Codec: capabilities.CodecH264, Accel: capabilities.AccelVideoToolbox}
	out := encode.SoftwareDecodeProfile(base)
	if out.ForceSoftware || out.Accel != capabilities.AccelNone || out.Codec != capabilities.CodecH264 {
		t.Fatalf("SoftwareDecodeProfile() = %+v", out)
	}
}

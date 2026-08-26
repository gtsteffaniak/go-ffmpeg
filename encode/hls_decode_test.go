package encode_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestSoftwareDecodeProfile(t *testing.T) {
	t.Parallel()
	base := encode.VideoDecodeProfile{Codec: capabilities.CodecH264, Accel: capabilities.AccelVideoToolbox}
	out := encode.SoftwareDecodeProfile(base)
	if out.ForceSoftware || out.Accel != capabilities.AccelNone || out.Codec != capabilities.CodecH264 {
		t.Fatalf("SoftwareDecodeProfile() = %+v", out)
	}
}

func TestMatchHLSTranscodeDecodeForcesCPUDecode(t *testing.T) {
	t.Parallel()
	decode := encode.HLSDecodeProfileForOnDemand(probe.StreamInfo{VideoCodec: "h264"})
	out := encode.MatchHLSTranscodeDecode(encode.VideoProfile{Codec: encode.CodecH264, ForceSoftware: true}, decode)
	if !out.ForceSoftware {
		t.Fatalf("MatchHLSTranscodeDecode() = %+v, want ForceSoftware", out)
	}
}

func TestHLSDecodeProfileForOnDemandWMV(t *testing.T) {
	t.Parallel()
	out := encode.HLSDecodeProfileForOnDemand(probe.StreamInfo{VideoCodec: "wmv3"})
	if !out.ForceSoftware || out.Codec != "" {
		t.Fatalf("wmv3 decode = %+v, want ForceSoftware without matrix codec", out)
	}
}

func TestIsKnownHLSInputVideoCodec(t *testing.T) {
	t.Parallel()
	if !encode.IsKnownHLSInputVideoCodec("h264") {
		t.Fatal("h264 should be known")
	}
	if encode.IsKnownHLSInputVideoCodec("wmv3") {
		t.Fatal("wmv3 should not be known")
	}
}

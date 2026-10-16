package ops

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestBuildHLSSegmentOptionsPropagatesInputVideoCodec(t *testing.T) {
	t.Parallel()
	params := HLSSegmentParams{
		InputVideoCodec: "hevc",
		OmitAudio:       true,
		Profile: encode.VideoProfile{
			Codec: encode.CodecH264,
		},
	}
	opts := BuildHLSSegmentOptions("/media/clip.mkv", 0, params, nil, nil, false, nil, DefaultHLSSegmentDurationSec)
	if opts.InputVideoCodec != "hevc" {
		t.Fatalf("InputVideoCodec = %q", opts.InputVideoCodec)
	}
	if !opts.VideoOnly {
		t.Fatal("expected VideoOnly from OmitAudio")
	}
}

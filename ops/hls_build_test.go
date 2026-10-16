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

func TestBuildHLSSegmentOptionsAddsStartTimeToInputSeek(t *testing.T) {
	t.Parallel()
	params := HLSSegmentParams{
		InputStartTimeSec: 1.459,
		Profile:           encode.VideoProfile{Codec: encode.CodecH264},
	}
	starts := []float64{0, 4}
	opts := BuildHLSSegmentOptions("/media/clip.ts", 0, params, starts, []float64{4, 4}, true, nil, DefaultHLSSegmentDurationSec)
	if opts.MediaTimelineSec != 0 {
		t.Fatalf("MediaTimelineSec = %v, want 0", opts.MediaTimelineSec)
	}
	if opts.StartSec < 1.458 || opts.StartSec > 1.460 {
		t.Fatalf("StartSec = %v, want ~1.459", opts.StartSec)
	}
	optsMid := BuildHLSSegmentOptions("/media/clip.ts", 1, params, starts, []float64{4, 4}, true, nil, DefaultHLSSegmentDurationSec)
	if optsMid.StartSec < 5.458 || optsMid.StartSec > 5.460 {
		t.Fatalf("StartSec = %v, want ~5.459", optsMid.StartSec)
	}
}

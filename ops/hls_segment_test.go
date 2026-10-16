package ops

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

func countSeekFlags(args []string) (beforeInput, afterInput int) {
	afterInputMarker := false
	for _, a := range args {
		if a == "-i" {
			afterInputMarker = true
			continue
		}
		if a != "-ss" {
			continue
		}
		if afterInputMarker {
			afterInput++
		} else {
			beforeInput++
		}
	}
	return beforeInput, afterInput
}

func TestBuildHLSSegmentTranscodeArgvSingleInputSeek(t *testing.T) {
	t.Parallel()
	caps := capabilities.NewCapabilities()
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Compiled: true, Available: true, Kind: "software"}
	caps.Decoders["h264"] = capabilities.DecoderCapability{Name: "h264", Compiled: true, Available: true, Kind: "software"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Software:       []string{"libx264"},
		SoftwareDecode: []string{"h264"},
		Preferred:      capabilities.EncoderSelection{Encoder: "libx264", Accel: capabilities.AccelNone, Kind: "software"},
		DecodePreferred: capabilities.DecoderSelection{
			Decoder: "h264",
			Accel:   capabilities.AccelNone,
			Kind:    "software",
			SWCodec: "h264",
		},
	}
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	opts := HLSSegmentOptions{
		Input:            InputSource{URL: "/media/clip.ts"},
		StartSec:         281.459,
		MediaTimelineSec: 280,
		AccurateSeek:     true,
		Profile:          encode.VideoProfile{Codec: encode.CodecH264},
		Decode:           encode.VideoDecodeProfile{Codec: capabilities.CodecH264},
		DurationSec:      4,
		GOP:              120,
	}
	args, err := buildHLSSegmentFFmpegArgs(runner, caps, opts, false)
	if err != nil {
		t.Fatalf("buildHLSSegmentFFmpegArgs: %v", err)
	}
	before, after := countSeekFlags(args)
	if before != 1 || after != 0 {
		t.Fatalf("expected one input -ss and no output -ss, got before=%d after=%d: %s", before, after, strings.Join(args, " "))
	}
}

package ops_test

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
)

func h264TranscodeCaps() *capabilities.Capabilities {
	caps := capabilities.NewCapabilities()
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Available: true, Kind: "software"}
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		Software:  []string{"libx264"},
		Preferred: capabilities.EncoderSelection{Encoder: "libx264", Accel: capabilities.AccelNone, Kind: "software"},
	}
	return caps
}

func TestBuildTranscodeArgsExtraOutputBeforeOutputPath(t *testing.T) {
	runner := &ffexec.Runner{}

	args, err := ops.BuildTranscodeArgs(runner, h264TranscodeCaps(), ops.TranscodeOptions{
		Input:      ops.InputSource{URL: "/tmp/in.mp4"},
		OutputPath: "/tmp/out.mp4",
		Profile: encode.VideoProfile{
			Codec:   encode.CodecH264,
			Quality: encode.PresetMedium,
			Bitrate: encode.BitrateConfig{Target: "4M", Max: "4M", BufSize: "8M", Min: "2M"},
		},
		ExtraOutputArgs: []string{"-movflags", "+faststart"},
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	outIdx := indexOf(args, "/tmp/out.mp4")
	movIdx := indexOf(args, "-movflags")
	if movIdx < 0 || outIdx < 0 || movIdx >= outIdx {
		t.Fatalf("expected -movflags before output path, args=%q", joined)
	}
	if indexOf(args, "-protocol_whitelist") < 0 {
		t.Fatalf("expected protocol whitelist, args=%q", joined)
	}
}

func TestBuildTranscodeArgsLegacyExtraArgs(t *testing.T) {
	runner := &ffexec.Runner{}

	args, err := ops.BuildTranscodeArgs(runner, h264TranscodeCaps(), ops.TranscodeOptions{
		Input:      ops.InputSource{URL: "/tmp/in.mp4"},
		OutputPath: "/tmp/out.mp4",
		Profile: encode.VideoProfile{
			Codec:   encode.CodecH264,
			Quality: encode.PresetMedium,
			Bitrate: encode.BitrateConfig{Target: "4M", Max: "4M", BufSize: "8M", Min: "2M"},
		},
		ExtraArgs: []string{"-max_muxing_queue_size", "1024"},
	})
	if err != nil {
		t.Fatal(err)
	}
	outIdx := indexOf(args, "/tmp/out.mp4")
	maxIdx := indexOf(args, "-max_muxing_queue_size")
	if maxIdx < 0 || maxIdx >= outIdx {
		t.Fatalf("legacy ExtraArgs should appear before output, args=%v", args)
	}
}

func indexOf(args []string, s string) int {
	for i, a := range args {
		if a == s {
			return i
		}
	}
	return -1
}

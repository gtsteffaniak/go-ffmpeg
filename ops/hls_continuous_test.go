package ops

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

func TestBuildHLSContinuousArgsUsesRelativeOutputPaths(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	caps := &capabilities.Capabilities{}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: "/media/video.mp4"},
		OutputDir:  "/cache/job",
		SegmentSec: 4,
		Remux:      true,
	}

	args, err := buildHLSContinuousArgs(runner, caps, opts)
	if err != nil {
		t.Fatalf("buildHLSContinuousArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-fflags +genpts",
		"-hls_segment_filename seg/%05d.m4s",
		"-hls_fmp4_init_filename init.m4s",
		"-avoid_negative_ts make_zero",
		"ffmpeg.m3u8",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "/cache/job/ffmpeg.m3u8") {
		t.Fatalf("expected relative playlist path, got absolute in args: %s", joined)
	}
}

func TestBuildHLSContinuousArgsReadrateWhenThrottleEnabled(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	caps := &capabilities.Capabilities{
		FeatureFlags: capabilities.FeatureFlags{
			Version:  capabilities.Version{Major: 8, Minor: 0, Patch: 0},
			Readrate: true,
		},
	}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: "/media/video.mkv"},
		OutputDir:  "/cache/job",
		SegmentSec: 4,
		Remux:      true,
		Throttle: encode.ThrottleConfig{
			Enabled:      true,
			Rate:         1.0,
			Catchup:      2.0,
			InitialBurst: 24,
		},
	}

	args, err := buildHLSContinuousArgs(runner, caps, opts)
	if err != nil {
		t.Fatalf("buildHLSContinuousArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-readrate 1",
		"-readrate_catchup 2",
		"-readrate_initial_burst 24",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
}

func TestBuildHLSContinuousTranscodeUsesHardwareDecode(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	caps := &capabilities.Capabilities{
		Encoders: map[string]capabilities.EncoderCapability{
			"h264_videotoolbox": {Name: "h264_videotoolbox", Available: true, Compiled: true},
		},
		Decoders: map[string]capabilities.DecoderCapability{
			"hwaccel:videotoolbox:h264": {Name: "hwaccel:videotoolbox:h264", Available: true},
		},
		Filters: map[string]bool{
			"scale_vt": true,
		},
		CodecMatrix: map[capabilities.VideoCodec]capabilities.CodecSupport{
			capabilities.CodecH264: {
				Preferred: capabilities.EncoderSelection{Encoder: "h264_videotoolbox", Accel: capabilities.AccelVideoToolbox},
				DecodePreferred: capabilities.DecoderSelection{
					Decoder: "hwaccel:videotoolbox:h264",
					Accel:   capabilities.AccelVideoToolbox,
					SWCodec: "h264",
				},
			},
		},
	}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: "/media/video.mkv"},
		OutputDir:  "/cache/job",
		SegmentSec: 4,
		StartSec:   3704,
		MaxHeight:  720,
		Profile:    encode.VideoProfile{Codec: encode.CodecH264},
		Decode:     encode.VideoDecodeProfile{Codec: capabilities.CodecH264},
	}

	args, err := buildHLSContinuousArgs(runner, caps, opts)
	if err != nil {
		t.Fatalf("buildHLSContinuousArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-init_hw_device videotoolbox=vt",
		"-hwaccel videotoolbox",
		"-hwaccel_output_format videotoolbox_vld",
		"-noautorotate",
		"+discardcorrupt",
		"-ss 3704.000",
		"-copyts",
		"-avoid_negative_ts disabled",
		"-hls_segment_options movflags=+frag_discont",
		"-max_delay 5000000",
		"scale_vt",
		"-qmin -1",
		"-allow_sw 1",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
	if !strings.Contains(joined, "videotoolbox_vld") {
		t.Fatalf("transcode must use hwaccel_output_format videotoolbox_vld for scale_vt: %s", joined)
	}
	if strings.Contains(joined, "-output_ts_offset") {
		t.Fatalf("transcode should not use output_ts_offset: %s", joined)
	}
}

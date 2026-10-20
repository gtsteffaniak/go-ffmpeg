package ops

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

func swH264Caps() *capabilities.Capabilities {
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
	return caps
}

func TestBuildHLSVODFFmpegArgsTranscodeMidFile(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	opts := HLSVODJobOptions{
		Input:      InputSource{URL: "/media/clip.mkv"},
		OutputDir:  "/cache/vod",
		StartIndex: 5,
		StartSec:   20,
		SegmentSec: 4,
		Profile:    encode.VideoProfile{Codec: encode.CodecH264},
		Decode:     encode.VideoDecodeProfile{Codec: capabilities.CodecH264},
		GOP:        120,
	}
	args, err := BuildHLSVODFFmpegArgs(runner, swH264Caps(), opts)
	if err != nil {
		t.Fatalf("BuildHLSVODFFmpegArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-ss 20.000",
		"-avoid_negative_ts disabled",
		"-output_ts_offset 20.000000",
		"-start_number 5",
		"-force_key_frames expr:gte(t,n_forced*4.000)",
		"-hls_segment_type fmp4",
		"-hls_segment_filename seg/%05d.m4s",
		"-hls_flags temp_file+independent_segments",
		".ffmpeg-vod.m3u8",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q: %s", want, joined)
		}
	}
	for _, bad := range []string{"-copyts", "max_interleave_delta", "async=192000", "raw/%05d.m4s", "frag_discont"} {
		if strings.Contains(joined, bad) {
			t.Fatalf("unexpected %q in %s", bad, joined)
		}
	}
	before, after := countSeekFlags(args)
	if before != 1 || after != 0 {
		t.Fatalf("seek flags before=%d after=%d", before, after)
	}
}

func TestBuildHLSVODFFmpegArgsTranscodeFractionalSegmentSec(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	opts := HLSVODJobOptions{
		Input:      InputSource{URL: "/media/clip.mkv"},
		OutputDir:  "/cache/vod",
		SegmentSec: 2.5,
		Profile:    encode.VideoProfile{Codec: encode.CodecH264},
		Decode:     encode.VideoDecodeProfile{Codec: capabilities.CodecH264},
	}
	args, err := BuildHLSVODFFmpegArgs(runner, swH264Caps(), opts)
	if err != nil {
		t.Fatalf("BuildHLSVODFFmpegArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-force_key_frames expr:gte(t,n_forced*2.500)",
		"-hls_time 2.500",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q: %s", want, joined)
		}
	}
}

func TestBuildHLSVODFFmpegArgsCopyUsesKeyframeSeek(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	opts := HLSVODJobOptions{
		Input:             InputSource{URL: "/media/clip.ts"},
		OutputDir:         "/cache/vod",
		StartIndex:        2,
		StartSec:          9,
		SegmentSec:        4,
		VideoCopy:         true,
		InputStartTimeSec: 1.5,
		KeyframeSeekTimes: []float64{0, 4, 8, 12, 16},
		InputVideoCodec:   "h264",
	}
	args, err := BuildHLSVODFFmpegArgs(runner, &capabilities.Capabilities{}, opts)
	if err != nil {
		t.Fatalf("BuildHLSVODFFmpegArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-ss 9.500") {
		t.Fatalf("expected input seek at keyframe 8 + start_time: %s", joined)
	}
	if !strings.Contains(joined, "-output_ts_offset 9.000000") {
		t.Fatalf("playlist offset must stay at segment start: %s", joined)
	}
}

func TestBuildHLSVODFFmpegArgsCopyExplicitSegmentTimes(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	kf := []float64{0, 6, 9, 12, 18, 21, 24}
	list := BuildVODSegmentListKeyframe(30, 4, 0, kf)
	opts := HLSVODJobOptions{
		Input:             InputSource{URL: "/media/clip.ts"},
		OutputDir:         "/cache/vod",
		StartIndex:        1,
		StartSec:          6,
		SegmentSec:        4,
		VideoCopy:         true,
		KeyframeSeekTimes: kf,
		SegmentList:       &list,
		InputVideoCodec:   "h264",
	}
	args, err := BuildHLSVODFFmpegArgs(runner, &capabilities.Capabilities{}, opts)
	if err != nil {
		t.Fatalf("BuildHLSVODFFmpegArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-ss 6.000",
		"-copyts",
		"-start_at_zero",
		"-f segment",
		"-segment_times 3.000,6.000,12.000,15.000,18.000",
		"-segment_start_number 1",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "-f hls") {
		t.Fatalf("explicit copy should use segment muxer, not hls: %s", joined)
	}
}

func TestBuildHLSVODFFmpegArgsRemux(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	opts := HLSVODJobOptions{
		Input:      InputSource{URL: "/media/clip.mp4"},
		OutputDir:  "/cache/vod",
		StartIndex: 0,
		SegmentSec: 4,
		Remux:      true,
	}
	args, err := BuildHLSVODFFmpegArgs(runner, &capabilities.Capabilities{}, opts)
	if err != nil {
		t.Fatalf("BuildHLSVODFFmpegArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-c:v copy") || !strings.Contains(joined, "-c:a copy") {
		t.Fatalf("remux codecs: %s", joined)
	}
}

func TestBuildHLSVODFFmpegArgsNVENC(t *testing.T) {
	t.Parallel()
	caps := swH264Caps()
	caps.Encoders["h264_nvenc"] = capabilities.EncoderCapability{Name: "h264_nvenc", Compiled: true, Available: true, Kind: "nvenc"}
	h264 := caps.CodecMatrix[capabilities.CodecH264]
	h264.Hardware = map[capabilities.AccelType]string{capabilities.AccelNVENC: "h264_nvenc"}
	h264.Preferred = capabilities.EncoderSelection{Encoder: "h264_nvenc", Accel: capabilities.AccelNVENC, Kind: "nvenc"}
	caps.CodecMatrix[capabilities.CodecH264] = h264

	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	opts := HLSVODJobOptions{
		Input:      InputSource{URL: "/media/clip.mkv"},
		OutputDir:  "/cache/vod",
		StartIndex: 3,
		StartSec:   12,
		SegmentSec: 4,
		Profile:    encode.VideoProfile{Codec: encode.CodecH264, Accel: capabilities.AccelNVENC},
		Decode:     encode.VideoDecodeProfile{Codec: capabilities.CodecH264},
	}
	args, err := BuildHLSVODFFmpegArgs(runner, caps, opts)
	if err != nil {
		t.Fatalf("BuildHLSVODFFmpegArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "h264_nvenc") {
		t.Fatalf("expected nvenc encoder: %s", joined)
	}
	if !strings.Contains(joined, "-output_ts_offset 12.000000") {
		t.Fatalf("missing output_ts_offset: %s", joined)
	}
}

func TestBuildHLSVODFFmpegArgsVAAPI(t *testing.T) {
	t.Parallel()
	caps := swH264Caps()
	caps.Encoders["h264_vaapi"] = capabilities.EncoderCapability{Name: "h264_vaapi", Compiled: true, Available: true, Kind: "vaapi"}
	caps.Filters["scale_vaapi"] = true
	h264 := caps.CodecMatrix[capabilities.CodecH264]
	h264.Hardware = map[capabilities.AccelType]string{capabilities.AccelVAAPI: "h264_vaapi"}
	h264.Preferred = capabilities.EncoderSelection{Encoder: "h264_vaapi", Accel: capabilities.AccelVAAPI, Kind: "vaapi"}
	caps.CodecMatrix[capabilities.CodecH264] = h264

	runner := &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}
	opts := HLSVODJobOptions{
		Input:      InputSource{URL: "/media/clip.mkv"},
		OutputDir:  "/cache/vod",
		StartIndex: 1,
		StartSec:   4,
		SegmentSec: 4,
		Profile:    encode.VideoProfile{Codec: encode.CodecH264, Accel: capabilities.AccelVAAPI},
		Decode:     encode.VideoDecodeProfile{Codec: capabilities.CodecH264},
	}
	args, err := BuildHLSVODFFmpegArgs(runner, caps, opts)
	if err != nil {
		t.Fatalf("BuildHLSVODFFmpegArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "h264_vaapi") {
		t.Fatalf("expected vaapi encoder: %s", joined)
	}
}

func TestStartHLSVODSegmentJobRejectsEmptyOutputDir(t *testing.T) {
	t.Parallel()
	_, err := StartHLSVODSegmentJob(t.Context(), &ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}, &capabilities.Capabilities{}, HLSVODJobOptions{
		Input: InputSource{URL: "/media/x.mp4"},
	})
	if err == nil || !strings.Contains(err.Error(), "output dir") {
		t.Fatalf("expected output dir error, got %v", err)
	}
}

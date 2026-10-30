package ops_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
)

// segmentCaps is h264TranscodeCaps plus a software h264 decoder, which the
// resolver expects when it validates an h264 profile.
func segmentCaps() *capabilities.Capabilities {
	caps := h264TranscodeCaps()
	support := caps.CodecMatrix[capabilities.CodecH264]
	support.DecodePreferred = capabilities.DecoderSelection{Decoder: "h264", Accel: capabilities.AccelNone, SWCodec: "h264"}
	caps.CodecMatrix[capabilities.CodecH264] = support
	return caps
}

// flagValue returns the argument following flag, or "" when absent.
func flagValue(args []string, flag string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func hasPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func probed(video, audio string) ops.SourceFacts {
	return ops.SourceFacts{Probed: true, HasVideo: true, HasAudio: audio != "", VideoCodec: video, AudioCodec: audio, BitDepth: 8}
}

func TestDecideSegmentPlanSoftwareDecodeWhenBitDepthUnknown(t *testing.T) {
	t.Parallel()
	opts := ops.SegmentRecordOptions{Mode: ops.RecordAuto, HardwareDecode: true}
	src := probed("mjpeg", "")
	src.BitDepth = 0
	if plan := ops.DecideSegmentPlan(opts, src); plan.HardwareDecode {
		t.Fatal("unknown bit depth must fall back to software decode")
	}
}

func TestDecideSegmentPlanVideoTable(t *testing.T) {
	t.Parallel()
	h264 := encode.VideoProfile{Codec: encode.CodecH264}
	hevc := encode.VideoProfile{Codec: encode.CodecHEVC}
	cases := []struct {
		name    string
		mode    ops.RecordMode
		profile encode.VideoProfile
		src     ops.SourceFacts
		want    string
		codec   string
	}{
		{"auto h264 source copies", ops.RecordAuto, encode.VideoProfile{}, probed("h264", "aac"), "copy", "h264"},
		{"auto hevc source keeps hevc", ops.RecordAuto, encode.VideoProfile{}, probed("hevc", "aac"), "copy", "hevc"},
		{"auto mjpeg source transcodes to h264", ops.RecordAuto, encode.VideoProfile{}, probed("mjpeg", ""), "transcode", "h264"},
		{"auto vp9 source is not copied into mp4", ops.RecordAuto, encode.VideoProfile{}, probed("vp9", "opus"), "transcode", "h264"},
		{"auto target matches source copies", ops.RecordAuto, h264, probed("h264", "aac"), "copy", "h264"},
		{"auto target differs transcodes", ops.RecordAuto, hevc, probed("h264", "aac"), "transcode", "hevc"},
		{"auto unprobed transcodes", ops.RecordAuto, encode.VideoProfile{}, ops.SourceFacts{}, "transcode", "h264"},
		{"transcode forced even for h264", ops.RecordTranscode, h264, probed("h264", "aac"), "transcode", "h264"},
		{"transcode zero profile is h264", ops.RecordTranscode, encode.VideoProfile{}, probed("h264", "aac"), "transcode", "h264"},
		{"copy forced", ops.RecordCopy, encode.VideoProfile{}, probed("mjpeg", ""), "copy", "mjpeg"},
		{"copy codec in profile is copy", ops.RecordTranscode, encode.VideoProfile{Codec: encode.CodecCopy}, probed("hevc", "aac"), "copy", "hevc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan := ops.DecideSegmentPlan(ops.SegmentRecordOptions{Mode: tc.mode, Profile: tc.profile}, tc.src)
			if plan.Video != tc.want {
				t.Fatalf("video = %q, want %q (reason: %s)", plan.Video, tc.want, plan.VideoReason)
			}
			if plan.VideoCodec != tc.codec {
				t.Fatalf("video codec = %q, want %q", plan.VideoCodec, tc.codec)
			}
			if plan.VideoReason == "" {
				t.Fatal("plan must explain the video decision")
			}
		})
	}
}

func TestDecideSegmentPlanAudioTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		mode  ops.RecordMode
		audio string
		src   ops.SourceFacts
		want  string
	}{
		{"auto aac source copies", ops.RecordAuto, ops.AudioAuto, probed("h264", "aac"), "copy"},
		{"auto mp3 source copies", ops.RecordAuto, ops.AudioAuto, probed("h264", "mp3"), "copy"},
		{"auto opus source re-encodes", ops.RecordAuto, ops.AudioAuto, probed("h264", "opus"), "aac"},
		{"auto unprobed uses aac", ops.RecordAuto, ops.AudioAuto, ops.SourceFacts{}, "aac"},
		{"auto no audio", ops.RecordAuto, ops.AudioAuto, probed("h264", ""), "none"},
		{"transcode default aac", ops.RecordTranscode, ops.AudioAuto, probed("h264", "aac"), "aac"},
		{"copy default copy", ops.RecordCopy, ops.AudioAuto, probed("h264", "opus"), "copy"},
		{"explicit aac wins over copy source", ops.RecordAuto, ops.AudioAAC, probed("h264", "aac"), "aac"},
		{"explicit copy wins over opus", ops.RecordTranscode, ops.AudioCopy, probed("h264", "opus"), "copy"},
		{"no audio beats explicit request", ops.RecordAuto, ops.AudioCopy, probed("h264", ""), "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan := ops.DecideSegmentPlan(ops.SegmentRecordOptions{Mode: tc.mode, AudioCodec: tc.audio}, tc.src)
			if plan.Audio != tc.want {
				t.Fatalf("audio = %q, want %q (reason: %s)", plan.Audio, tc.want, plan.AudioReason)
			}
		})
	}
}

func TestDecideSegmentPlanHardwareDecodeOnlyWhenTranscoding(t *testing.T) {
	t.Parallel()
	opts := ops.SegmentRecordOptions{Mode: ops.RecordAuto, HardwareDecode: true}
	if plan := ops.DecideSegmentPlan(opts, probed("h264", "aac")); plan.HardwareDecode {
		t.Fatal("copy must not request hardware decode")
	}
	if plan := ops.DecideSegmentPlan(opts, probed("mjpeg", "")); !plan.HardwareDecode {
		t.Fatal("transcode of a probed source should request hardware decode")
	}
	if plan := ops.DecideSegmentPlan(opts, ops.SourceFacts{}); plan.HardwareDecode {
		t.Fatal("unprobed source has no decode codec")
	}
}

func TestValidateAlignedSegmentDuration(t *testing.T) {
	t.Parallel()
	_, err := ops.BuildSegmentRecordArgs(&ffexec.Runner{}, segmentCaps(), ops.SegmentRecordOptions{
		Input:                ops.InputSource{URL: "rtsp://cam/stream"},
		OutputDir:            "/tmp/rec",
		SegmentDuration:      7 * time.Hour,
		AlignSegmentsToClock: true,
		Profile:              encode.VideoProfile{Codec: encode.CodecH264},
	}, ops.SegmentPlan{Video: "transcode", VideoCodec: "h264", Audio: "aac", SegmentSeconds: 7 * 3600, Aligned: true})
	if err == nil {
		t.Fatal("7h segments must not be clock aligned: 24h is not a multiple")
	}
}

func TestBuildSegmentRecordArgsDefaultsForTranscode(t *testing.T) {
	t.Parallel()
	opts := ops.SegmentRecordOptions{
		Input:     ops.InputSource{URL: "rtsp://cam/stream"},
		OutputDir: "/tmp/rec",
		Profile:   encode.VideoProfile{Codec: encode.CodecH264, Bitrate: encode.BitrateConfig{Target: "2M"}},
	}
	plan := ops.DecideSegmentPlan(opts, ops.SourceFacts{})
	args, err := ops.BuildSegmentRecordArgs(&ffexec.Runner{}, segmentCaps(), opts, plan)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, banned := range []string{"-segment_atclocktime", "movflags", "-map 0 ", "-c:v copy", "wrap_duration", "break_non_keyframes"} {
		if strings.Contains(joined, banned) {
			t.Fatalf("default plan must not emit %q: %s", banned, joined)
		}
	}
	if !hasPair(args, "-map", "0:v:0") || !hasPair(args, "-map", "0:a:0?") {
		t.Fatalf("expected explicit stream selection: %v", args)
	}
	if got := flagValue(args, "-segment_time"); got != "3600" {
		t.Fatalf("segment_time = %q, want 3600", got)
	}
	if got := flagValue(args, "-segment_list_flags"); got != "+live" {
		t.Fatalf("segment list must be live, got %q", got)
	}
	if got := flagValue(args, "-b:a"); got != "128k" {
		t.Fatalf("audio bitrate = %q, want 128k", got)
	}
	if got := args[len(args)-1]; got != "/tmp/rec/%Y-%m-%d_%Hh%Mm_seg.mp4" {
		t.Fatalf("output pattern = %q", got)
	}
}

func TestBuildSegmentRecordArgsClockAlignedFragmentedCopy(t *testing.T) {
	t.Parallel()
	opts := ops.SegmentRecordOptions{
		Input:                ops.InputSource{URL: "rtsp://cam/stream"},
		OutputDir:            "/tmp/rec",
		SegmentPattern:       "%Y-%m-%d_%Hh%Mm_cam_temp.mp4",
		SegmentListPath:      "/tmp/rec/cam_segments.ffcat",
		SegmentDuration:      time.Hour,
		Mode:                 ops.RecordAuto,
		AlignSegmentsToClock: true,
		Fragmented:           true,
	}
	src := probed("h264", "aac")
	plan := ops.DecideSegmentPlan(opts, src)
	args, err := ops.BuildSegmentRecordArgs(&ffexec.Runner{}, segmentCaps(), opts, plan)
	if err != nil {
		t.Fatal(err)
	}
	if got := flagValue(args, "-c:v"); got != "copy" {
		t.Fatalf("-c:v = %q, want copy", got)
	}
	if got := flagValue(args, "-c:a"); got != "copy" {
		t.Fatalf("-c:a = %q, want copy for AAC source", got)
	}
	if strings.Contains(strings.Join(args, " "), "libx264") {
		t.Fatalf("copy plan must not select an encoder: %v", args)
	}
	checks := map[string]string{
		"-segment_atclocktime":    "1",
		"-segment_format_options": "movflags=+frag_keyframe+empty_moov+default_base_moof",
		"-segment_list":           "/tmp/rec/cam_segments.ffcat",
		"-segment_list_type":      "ffconcat",
	}
	for flag, want := range checks {
		if got := flagValue(args, flag); got != want {
			t.Errorf("%s = %q, want %q", flag, got, want)
		}
	}
	if got := args[len(args)-1]; got != "/tmp/rec/%Y-%m-%d_%Hh%Mm_cam_temp.mp4" {
		t.Errorf("output path = %q", got)
	}
}

func TestBuildSegmentRecordArgsHardwareDecodeUsesSourceCodec(t *testing.T) {
	t.Parallel()
	caps := segmentCaps()
	opts := ops.SegmentRecordOptions{
		Input:          ops.InputSource{URL: "rtsp://cam/stream"},
		OutputDir:      "/tmp/rec",
		Mode:           ops.RecordTranscode,
		Profile:        encode.VideoProfile{Codec: encode.CodecH264},
		HardwareDecode: true,
	}
	plan := ops.DecideSegmentPlan(opts, probed("mjpeg", ""))
	plan.SourceCodec = "mjpeg"
	// Must not panic or fail even with no hardware decoder in caps: software decode is the fallback.
	if _, err := ops.BuildSegmentRecordArgs(&ffexec.Runner{}, caps, opts, plan); err != nil {
		t.Fatalf("hardware decode must fall back silently: %v", err)
	}
}

func TestBuildSegmentRecordArgsRejectsUnknownAudioCodec(t *testing.T) {
	t.Parallel()
	_, err := ops.BuildSegmentRecordArgs(&ffexec.Runner{}, segmentCaps(), ops.SegmentRecordOptions{
		Input:      ops.InputSource{URL: "rtsp://cam/stream"},
		OutputDir:  "/tmp/rec",
		AudioCodec: "flac",
	}, ops.SegmentPlan{Video: "copy", VideoCodec: "h264", Audio: "copy", SegmentSeconds: 3600})
	if err == nil {
		t.Fatal("expected error for unsupported audio codec")
	}
}

func TestSegmentPlanStringIsLogFriendly(t *testing.T) {
	t.Parallel()
	plan := ops.DecideSegmentPlan(ops.SegmentRecordOptions{Mode: ops.RecordAuto}, probed("h264", "aac"))
	s := plan.String()
	for _, want := range []string{"mode=auto", "video=copy", "audio=copy", "segments=3600s"} {
		if !strings.Contains(s, want) {
			t.Fatalf("plan string %q missing %q", s, want)
		}
	}
}

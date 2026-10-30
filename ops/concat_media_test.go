package ops_test

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
)

func TestDecideConcatPlan(t *testing.T) {
	t.Parallel()
	a := ops.SourceFacts{Probed: true, HasVideo: true, HasAudio: true, VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080}
	b := a
	b.Width = 1280
	noAudio := a
	noAudio.HasAudio = false
	noAudio.AudioCodec = ""
	unprobed := ops.SourceFacts{}

	cases := []struct {
		name  string
		opts  ops.ConcatMediaOptions
		facts []ops.SourceFacts
		want  string
	}{
		{"identical inputs copy", ops.ConcatMediaOptions{Inputs: []string{"a", "b"}}, []ops.SourceFacts{a, a}, ops.ConcatActionCopy},
		{"dimension mismatch transcodes", ops.ConcatMediaOptions{Inputs: []string{"a", "b"}}, []ops.SourceFacts{a, b}, ops.ConcatActionTranscode},
		{"audio presence mismatch transcodes", ops.ConcatMediaOptions{Inputs: []string{"a", "b"}}, []ops.SourceFacts{a, noAudio}, ops.ConcatActionTranscode},
		{"unprobed input transcodes", ops.ConcatMediaOptions{Inputs: []string{"a", "b"}}, []ops.SourceFacts{a, unprobed}, ops.ConcatActionTranscode},
		{"fps change forces transcode", ops.ConcatMediaOptions{Inputs: []string{"a"}, FPS: 24}, []ops.SourceFacts{a}, ops.ConcatActionTranscode},
		{"rescale forces transcode", ops.ConcatMediaOptions{Inputs: []string{"a"}, Width: 640, Height: 360}, []ops.SourceFacts{a}, ops.ConcatActionTranscode},
		{"no inputs transcodes", ops.ConcatMediaOptions{}, nil, ops.ConcatActionTranscode},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan := ops.DecideConcatPlan(tc.opts, tc.facts)
			if plan.Action != tc.want {
				t.Fatalf("action = %q, want %q (reason: %s)", plan.Action, tc.want, plan.Reason)
			}
			if plan.Reason == "" {
				t.Fatal("plan must explain the decision")
			}
		})
	}
}

func TestBuildConcatListEscapesQuotes(t *testing.T) {
	t.Parallel()
	got := ops.BuildConcatList([]string{"/data/a.mp4", "/data/it's here.mp4"})
	want := "file '/data/a.mp4'\nfile '/data/it'\\''s here.mp4'\n"
	if got != want {
		t.Fatalf("list = %q, want %q", got, want)
	}
}

func TestBuildConcatMediaArgsCopy(t *testing.T) {
	t.Parallel()
	args, err := ops.BuildConcatMediaArgs(&ffexec.Runner{}, segmentCaps(), ops.ConcatMediaOptions{OutputPath: "/tmp/out.mp4"},
		ops.ConcatPlan{Action: ops.ConcatActionCopy}, "/tmp/list.txt")
	if err != nil {
		t.Fatal(err)
	}
	if flagValue(args, "-f") != "concat" || flagValue(args, "-safe") != "0" || flagValue(args, "-i") != "/tmp/list.txt" {
		t.Fatalf("missing concat demuxer flags: %v", args)
	}
	if flagValue(args, "-c") != "copy" {
		t.Fatalf("copy plan must stream copy: %v", args)
	}
	if strings.Contains(strings.Join(args, " "), "libx264") {
		t.Fatalf("copy plan must not re-encode: %v", args)
	}
	if args[len(args)-1] != "/tmp/out.mp4" {
		t.Fatalf("output must be last: %v", args)
	}
}

func TestBuildConcatMediaArgsTranscodeWithFPSAndScale(t *testing.T) {
	t.Parallel()
	opts := ops.ConcatMediaOptions{
		OutputPath: "/tmp/out.mp4",
		Profile:    encode.VideoProfile{Codec: encode.CodecH264},
		Width:      640,
		Height:     360,
		FPS:        12.5,
	}
	args, err := ops.BuildConcatMediaArgs(&ffexec.Runner{}, segmentCaps(), opts, ops.ConcatPlan{Action: ops.ConcatActionTranscode}, "/tmp/list.txt")
	if err != nil {
		t.Fatal(err)
	}
	if flagValue(args, "-vf") != "scale=640:360" {
		t.Fatalf("missing scale: %v", args)
	}
	if flagValue(args, "-r") != "12.500" {
		t.Fatalf("fps = %q", flagValue(args, "-r"))
	}
	if !strings.Contains(strings.Join(args, " "), "-c:v libx264") {
		t.Fatalf("expected libx264 from resolver: %v", args)
	}
}

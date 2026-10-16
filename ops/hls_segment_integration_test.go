//go:build integration

package ops

import (
	"context"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/mp4"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestHLSSegmentTranscodeSeekLatencyNearFlat(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	input := filepath.Join(t.TempDir(), "long.mkv")
	generateIntegrationH264MKV(t, ffmpegPath, input, 300)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{SkipHWTests: true})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	keyframes, err := ProbeVideoKeyframeTimes(ctx, runner, input)
	if err != nil {
		t.Fatalf("keyframes: %v", err)
	}
	keyframeSeekTimes, mediaDur := PrepareHLSMediaTimeline(keyframes, 300, 0)
	starts, durations := BuildHLSSegmentTimeline(mediaDur, keyframeSeekTimes, DefaultHLSSegmentDurationSec, 0)
	params := HLSSegmentParams{
		Profile: encode.DefaultHLSVideoProfile(720),
		Decode:  encode.VideoDecodeProfile{Codec: capabilities.CodecH264, Accel: capabilities.AccelNone},
		GOP:     120,
	}
	early := len(starts) / 10
	late := len(starts) * 9 / 10
	if late <= early {
		t.Fatalf("timeline too short: %d segments", len(starts))
	}

	encodeSegment := func(index int) time.Duration {
		opts := BuildHLSSegmentOptions(input, index, params, starts, durations, true, keyframeSeekTimes, DefaultHLSSegmentDurationSec)
		start := time.Now()
		_, err := runHLSSegmentRaw(ctx, runner, caps, opts, false)
		if err != nil {
			t.Fatalf("segment %d: %v", index, err)
		}
		return time.Since(start)
	}

	earlyDur := encodeSegment(early)
	lateDur := encodeSegment(late)
	if lateDur > earlyDur*2 {
		t.Fatalf("seek latency regressed: early=%v late=%v (%.1fx)", earlyDur, lateDur, float64(lateDur)/float64(earlyDur))
	}
}

func TestHLSSegmentMPEGTSStartTimeMediaOrigin(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	input := filepath.Join(t.TempDir(), "offset.ts")
	generateIntegrationMPEGTS(t, ffmpegPath, input, 32)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{SkipHWTests: true})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	info, err := probe.ProbeStream(ctx, runner, probe.ProbeStreamOptions{URL: input, StreamType: probe.StreamFile})
	if err != nil || info.StartTime <= 0 {
		t.Fatalf("probe start_time: info=%+v err=%v", info, err)
	}

	keyframes, err := ProbeVideoKeyframeTimes(ctx, runner, input)
	if err != nil {
		t.Fatalf("keyframes: %v", err)
	}
	keyframeSeekTimes, mediaDur := PrepareHLSMediaTimeline(keyframes, info.Duration, info.StartTime)
	starts, durations := BuildHLSSegmentTimeline(mediaDur, keyframeSeekTimes, DefaultHLSSegmentDurationSec, 0)
	if len(starts) == 0 || starts[0] > 0.05 {
		t.Fatalf("first segment start = %.3f, want media 0", starts[0])
	}

	params := HLSSegmentParams{
		Profile:           encode.DefaultHLSVideoProfile(720),
		Decode:            encode.VideoDecodeProfile{Codec: capabilities.CodecH264, Accel: capabilities.AccelNone},
		GOP:               120,
		InputStartTimeSec: info.StartTime,
	}
	mid := len(starts) / 2
	if mid < 1 {
		mid = 1
	}
	for _, index := range []int{0, mid} {
		opts := BuildHLSSegmentOptions(input, index, params, starts, durations, true, keyframeSeekTimes, DefaultHLSSegmentDurationSec)
		raw, err := runHLSSegmentRaw(ctx, runner, caps, opts, true)
		if err != nil {
			t.Fatalf("segment %d: %v", index, err)
		}
		_, media, err := mp4.SplitInitMedia(raw)
		if err != nil {
			t.Fatalf("split: %v", err)
		}
		media, err = finalizeHLSSegmentMedia(media, opts)
		if err != nil {
			t.Fatalf("finalize: %v", err)
		}
		got, err := mp4.FragmentMediaStartSec(media)
		if err != nil {
			t.Fatalf("tfdt: %v", err)
		}
		want := starts[index]
		if math.Abs(got-want) > 0.15 {
			t.Fatalf("segment %d tfdt start %.3f, want %.3f", index, got, want)
		}
	}
}

func TestContinuousVideoCopyStartSnapsToKeyframe(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	input := filepath.Join(t.TempDir(), "input.mkv")
	generateIntegrationH264MKV(t, ffmpegPath, input, 32)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx := context.Background()
	keyframes, err := ProbeVideoKeyframeTimes(ctx, runner, input)
	if err != nil {
		t.Fatalf("keyframes: %v", err)
	}
	norm := NormalizeHLSKeyframeTimes(keyframes, 0)
	const mediaStart = 5.5
	wantSnap := KeyframeSeekBefore(norm, mediaStart)
	if math.Abs(wantSnap-4.0) > 0.1 {
		t.Fatalf("expected snap near 4.0s, got %.3f", wantSnap)
	}

	opts := HLSContinuousOptions{
		Input:      InputSource{URL: input, StreamType: probe.StreamFile},
		OutputDir:  t.TempDir(),
		SegmentSec: 4,
		VideoCopy:  true,
		StartSec:   mediaStart,
	}
	opts, err = snapContinuousCopyStart(ctx, runner, opts)
	if err != nil {
		t.Fatalf("snap: %v", err)
	}
	if math.Abs(opts.StartSec-wantSnap) > 0.001 {
		t.Fatalf("StartSec = %.3f, want snapped %.3f", opts.StartSec, wantSnap)
	}
}

func generateIntegrationMPEGTS(t *testing.T, ffmpegPath, outPath string, durationSec int) {
	t.Helper()
	cmd := exec.Command(ffmpegPath,
		"-hide_banner", "-y",
		"-f", "lavfi", "-i", "testsrc=duration="+strconv.Itoa(durationSec)+":size=1280x720:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440:duration="+strconv.Itoa(durationSec),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-g", "48", "-keyint_min", "48",
		"-preset", "veryfast", "-crf", "23",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-f", "mpegts",
		outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate mpegts: %v\n%s", err, out)
	}
}

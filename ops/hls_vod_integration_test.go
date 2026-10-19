//go:build integration

package ops

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/mp4"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestHLSVODTranscodeMidFileTimeline(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	input := filepath.Join(t.TempDir(), "vod.mkv")
	generateIntegrationH264MKV(t, ffmpegPath, input, 32)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{SkipHWTests: true})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	list := BuildVODSegmentListFixedGrid(32, DefaultHLSSegmentDurationSec)
	const startIndex = 4
	startSec := list.Starts[startIndex]
	outDir := t.TempDir()

	opts := HLSVODJobOptions{
		Input:      InputSource{URL: input, StreamType: probe.StreamFile},
		OutputDir:  outDir,
		StartIndex: startIndex,
		StartSec:   startSec,
		SegmentSec: DefaultHLSSegmentDurationSec,
		Profile:    encode.DefaultHLSVideoProfile(720),
		Decode:     encode.VideoDecodeProfile{Codec: capabilities.CodecH264, Accel: capabilities.AccelNone},
		GOP:        120,
	}
	job, err := StartHLSVODSegmentJob(ctx, runner, caps, opts)
	if err != nil {
		t.Fatalf("StartHLSVODSegmentJob: %v", err)
	}

	// HLS fMP4 segment files may report shorter first-fragment spans than #EXTINF; wait for a stable moof.
	waitReady := func(path string) []byte {
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			ok, err := HLSVODSegmentReady(path)
			if err != nil {
				t.Fatalf("ready %s: %v", path, err)
			}
			if ok {
				data, err := os.ReadFile(path)
				if err == nil {
					return data
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatalf("timeout waiting for ready segment %s", path)
		return nil
	}
	seg0 := waitReady(HLSVODSegmentPath(outDir, startIndex))
	seg1 := waitReady(HLSVODSegmentPath(outDir, startIndex+1))
	job.Cancel()
	_ = job.Wait()

	initData, err := os.ReadFile(HLSVODInitPath(outDir))
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	trackTS := mp4.TrackTimescalesFromInit(initData)
	segments := [][]byte{seg0, seg1}

	for i, media := range segments {
		idx := startIndex + i
		dur := mp4.FragmentDurationSec(media)
		if dur <= 0 {
			t.Fatalf("segment %d: unreadable duration", idx)
		}
		start, err := mp4.FragmentMediaStartSecWithTimescales(media, trackTS)
		if err != nil {
			t.Fatalf("segment %d tfdt: %v", idx, err)
		}
		if i == 0 && start > 2.0 {
			t.Fatalf("first emitted segment %d tfdt start %.3f, want near 0 after mid-file seek", idx, start)
		}
	}
}

func TestHLSVODCopyMidFileSegmentDurationsMatchPlan(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	input := filepath.Join(t.TempDir(), "vod-gop3.mp4")
	generateIntegrationH264MP4GOP(t, ffmpegPath, input, 30, 72)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{SkipHWTests: true})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	kf, err := ProbeVideoKeyframeTimes(ctx, runner, input)
	if err != nil {
		t.Fatalf("keyframes: %v", err)
	}
	const segmentSec = 4.0
	list := BuildVODSegmentListKeyframe(30, segmentSec, 0, kf)
	if list.Count() < 3 {
		t.Fatalf("segment list too short: %d", list.Count())
	}
	const startIndex = 1
	startSec := list.Starts[startIndex]
	outDir := t.TempDir()

	opts := HLSVODJobOptions{
		Input:                  InputSource{URL: input, StreamType: probe.StreamFile},
		OutputDir:              outDir,
		StartIndex:             startIndex,
		StartSec:               startSec,
		SegmentSec:             segmentSec,
		VideoCopy:              true,
		InputVideoCodec:        "h264",
		KeyframeSeekTimes:      kf,
		ClearSegmentsFromIndex: true,
	}
	job, err := StartHLSVODSegmentJob(ctx, runner, caps, opts)
	if err != nil {
		t.Fatalf("StartHLSVODSegmentJob: %v", err)
	}
	if job.StartIndex() != 0 {
		t.Fatalf("copy job StartIndex=%d, want 0 after normalization", job.StartIndex())
	}

	waitReady := func(index int) {
		path := HLSVODSegmentPath(outDir, index)
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			ok, err := HLSVODSegmentReady(path)
			if err != nil {
				t.Fatalf("ready %s: %v", path, err)
			}
			if ok {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatalf("timeout waiting for segment %d", index)
	}
	for i := 0; i <= startIndex+1 && i < list.Count(); i++ {
		waitReady(i)
	}
	job.Cancel()
	_ = job.Wait()

	extinf, err := parseInternalVODPlaylistEXTINF(filepath.Join(outDir, HLSVODInternalPlaylistName))
	if err != nil {
		t.Fatalf("internal playlist: %v", err)
	}
	const tol = mp4.DefaultHLSTimeToleranceSec
	for i := startIndex; i <= startIndex+1 && i < list.Count(); i++ {
		if i >= len(extinf) {
			t.Fatalf("playlist missing EXTINF for segment %d", i)
		}
		if math.Abs(extinf[i]-list.Durations[i]) > tol {
			t.Fatalf("segment %d EXTINF %.3f != planned %.3f (copy mid-file seek must match absolute grid)", i, extinf[i], list.Durations[i])
		}
	}
}

func generateIntegrationH264MP4GOP(t *testing.T, ffmpegPath, outPath string, durationSec, gop int) {
	t.Helper()
	runFFmpeg(t, ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration="+strconv.Itoa(durationSec)+":size=320x240:rate=24",
		"-c:v", "libx264", "-g", strconv.Itoa(gop), "-keyint_min", strconv.Itoa(gop), "-pix_fmt", "yuv420p",
		outPath)
}

func parseInternalVODPlaylistEXTINF(path string) ([]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []float64
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#EXTINF:") {
			continue
		}
		val := strings.TrimPrefix(line, "#EXTINF:")
		val = strings.TrimSuffix(val, ",")
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

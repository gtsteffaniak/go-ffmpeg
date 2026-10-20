//go:build integration

package ops

import (
	"context"
	"math"
	"os"
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

	outFrom0 := t.TempDir()
	outRestart := t.TempDir()

	baseOpts := HLSVODJobOptions{
		Input:             InputSource{URL: input, StreamType: probe.StreamFile},
		SegmentSec:        segmentSec,
		VideoCopy:         true,
		InputVideoCodec:   "h264",
		KeyframeSeekTimes: kf,
		SegmentList:       &list,
	}

	from0Opts := baseOpts
	from0Opts.OutputDir = outFrom0
	from0Opts.StartIndex = 0
	from0Opts.StartSec = 0

	restartOpts := baseOpts
	restartOpts.OutputDir = outRestart
	restartOpts.StartIndex = startIndex
	restartOpts.StartSec = startSec
	restartOpts.ClearSegmentsFromIndex = true

	job0, err := StartHLSVODSegmentJob(ctx, runner, caps, from0Opts)
	if err != nil {
		t.Fatalf("from0 job: %v", err)
	}
	job1, err := StartHLSVODSegmentJob(ctx, runner, caps, restartOpts)
	if err != nil {
		t.Fatalf("restart job: %v", err)
	}
	if job1.StartIndex() != startIndex {
		t.Fatalf("copy restart StartIndex=%d want %d", job1.StartIndex(), startIndex)
	}

	waitReady := func(outDir string, index int) {
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
		t.Fatalf("timeout waiting for segment %d in %s", index, outDir)
	}
	for i := 0; i <= startIndex+2 && i < list.Count(); i++ {
		waitReady(outFrom0, i)
	}
	for i := startIndex; i <= startIndex+2 && i < list.Count(); i++ {
		waitReady(outRestart, i)
	}
	job0.Cancel()
	job1.Cancel()
	_ = job0.Wait()
	_ = job1.Wait()

	init0, _ := os.ReadFile(HLSVODInitPath(outFrom0))
	init1, _ := os.ReadFile(HLSVODInitPath(outRestart))
	ts0 := mp4.TrackTimescalesFromInit(init0)
	ts1 := mp4.TrackTimescalesFromInit(init1)
	const tol = mp4.DefaultHLSTimeToleranceSec

	for i := startIndex; i <= startIndex+2 && i < list.Count(); i++ {
		p0 := HLSVODSegmentPath(outFrom0, i)
		p1 := HLSVODSegmentPath(outRestart, i)
		b0, err := os.ReadFile(p0)
		if err != nil {
			t.Fatal(err)
		}
		b1, err := os.ReadFile(p1)
		if err != nil {
			t.Fatal(err)
		}
		d0 := mp4.FragmentDurationSecWithTimescales(b0, ts0)
		d1 := mp4.FragmentDurationSecWithTimescales(b1, ts1)
		if math.Abs(d0-list.Durations[i]) > tol || math.Abs(d1-list.Durations[i]) > tol {
			t.Fatalf("segment %d dur from0=%.3f restart=%.3f planned=%.3f", i, d0, d1, list.Durations[i])
		}
		if math.Abs(d0-d1) > tol {
			t.Fatalf("segment %d duration mismatch from0=%.3f restart=%.3f", i, d0, d1)
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

func BenchmarkHLSVODCopySeekToTail(b *testing.B) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(&testing.T{})
	input := filepath.Join(b.TempDir(), "vod-long.mp4")
	generateIntegrationH264MP4GOP(&testing.T{}, ffmpegPath, input, 120, 72)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx := context.Background()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{SkipHWTests: true})
	if err != nil {
		b.Fatalf("Detect: %v", err)
	}
	kf, err := ProbeVideoKeyframeTimes(ctx, runner, input)
	if err != nil {
		b.Fatalf("keyframes: %v", err)
	}
	list := BuildVODSegmentListKeyframe(120, 4, 0, kf)
	k := int(float64(list.Count()) * 0.9)
	if k >= list.Count() {
		k = list.Count() - 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		outDir := b.TempDir()
		opts := HLSVODJobOptions{
			Input:             InputSource{URL: input, StreamType: probe.StreamFile},
			OutputDir:         outDir,
			StartIndex:        k,
			StartSec:          list.Starts[k],
			SegmentSec:        4,
			VideoCopy:         true,
			InputVideoCodec:   "h264",
			KeyframeSeekTimes: kf,
			SegmentList:       &list,
		}
		b.StartTimer()
		start := time.Now()
		job, err := StartHLSVODSegmentJob(ctx, runner, caps, opts)
		if err != nil {
			b.Fatalf("job: %v", err)
		}
		path := HLSVODSegmentPath(outDir, k)
		deadline := time.Now().Add(2 * time.Minute)
		for time.Now().Before(deadline) {
			ok, err := HLSVODSegmentReady(path)
			if err != nil {
				b.Fatalf("ready: %v", err)
			}
			if ok {
				b.StopTimer()
				b.ReportMetric(float64(time.Since(start).Milliseconds()), "ms/first-seg")
				job.Cancel()
				_ = job.Wait()
				b.StartTimer()
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

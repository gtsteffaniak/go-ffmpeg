//go:build integration

package ops

import (
	"context"
	"os"
	"path/filepath"
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

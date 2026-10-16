//go:build integration

package ops

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestContinuousSoftwareTranscodeFullFile(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	input := filepath.Join(t.TempDir(), "input.mkv")
	generateIntegrationH264MKV(t, ffmpegPath, input, 32)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{SkipHWTests: true})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "job")
	if err := os.MkdirAll(filepath.Join(outDir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: input, StreamType: probe.StreamFile},
		OutputDir:  outDir,
		SegmentSec: 4,
		MaxHeight:  720,
		Profile:    encode.DefaultHLSVideoProfile(720),
		Decode:     encode.VideoDecodeProfile{Codec: capabilities.CodecH264, Accel: capabilities.AccelNone},
	}
	job, err := StartHLSContinuous(ctx, runner, caps, opts)
	if err != nil {
		t.Fatalf("StartHLSContinuous: %v", err)
	}
	if err := job.Wait(); err != nil {
		t.Fatalf("job wait: %v", err)
	}
	if err := AlignContinuousHLSSegments(outDir, opts); err != nil {
		t.Fatalf("AlignContinuousHLSSegments: %v", err)
	}

	result, err := ValidateContinuousHLSOutput(outDir, opts, true, 0.15)
	if err != nil {
		t.Fatalf("ValidateContinuousHLSOutput: %v", err)
	}
	if result.SegmentCount < 7 {
		t.Fatalf("segment count = %d, want >= 7 for 32s @ 4s", result.SegmentCount)
	}
	if !result.HasEndList {
		t.Fatal("missing #EXT-X-ENDLIST")
	}
	if len(result.Issues) > 0 {
		t.Fatalf("validation issues: %+v", result.Issues)
	}
}

// A continuous job must publish its playlist as segments complete, not only at
// EOF. With -hls_playlist_type vod ffmpeg wrote the playlist only when the stream
// ended, so a long transcode served nothing until it finished - and for a
// feature-length input that never happened.
func TestContinuousSoftwarePublishesPlaylistDuringRun(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	input := filepath.Join(t.TempDir(), "input.mkv")
	generateIntegrationH264MKV(t, ffmpegPath, input, 60)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{SkipHWTests: true})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "job")
	if err := os.MkdirAll(filepath.Join(outDir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: input, StreamType: probe.StreamFile},
		OutputDir:  outDir,
		SegmentSec: 4,
		MaxHeight:  720,
		Profile:    encode.DefaultHLSVideoProfile(720),
		Decode:     encode.VideoDecodeProfile{Codec: capabilities.CodecH264, Accel: capabilities.AccelNone},
	}
	job, err := StartHLSContinuous(ctx, runner, caps, opts)
	if err != nil {
		t.Fatalf("StartHLSContinuous: %v", err)
	}
	defer func() {
		job.Cancel()
		_ = job.Wait()
	}()

	// Wait for the first completed segment, then require the playlist to be
	// published while the job is still running. The .m4s file appears when the
	// muxer starts a segment but the playlist only lists it once the segment
	// closes, so poll for the playlist rather than reading it immediately.
	firstSegment := filepath.Join(outDir, "seg", "00000.m4s")
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(firstSegment); statErr == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, statErr := os.Stat(firstSegment); statErr != nil {
		t.Fatalf("first segment never completed: %v", statErr)
	}

	playlistPath := filepath.Join(outDir, "ffmpeg.m3u8")
	jobDone := job.Done()
	var playlist []byte
	for time.Now().Before(deadline) {
		if data, readErr := os.ReadFile(playlistPath); readErr == nil && strings.Contains(string(data), "00000.m4s") {
			playlist = data
			break
		}
		select {
		case err := <-jobDone:
			t.Fatalf("job finished before publishing a playlist: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
	}
	if playlist == nil {
		t.Fatal("playlist not published while the job was still running")
	}
	if !strings.Contains(string(playlist), "00000.m4s") {
		t.Fatalf("playlist does not list the first segment:\n%s", playlist)
	}
}

// A seeking job keeps source timestamps on the video (-copyts), so the audio must
// not be forced back to the media origin. Doing so leaves the first audio packet
// at ~0 while the video stays at the seek offset, i.e. an A/V skew as long as the
// offset. The continuous aligner rebases both tracks to the HLS media grid, which
// hides it for callers that align, so this drives the argv directly and checks the
// raw muxer output.
func TestContinuousSoftwareTranscodeFromSeekOffset(t *testing.T) {
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	input := filepath.Join(t.TempDir(), "input.mkv")
	generateIntegrationH264MKV(t, ffmpegPath, input, 32)

	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{SkipHWTests: true})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	const segmentSec = 4.0
	const startIndex = 3 // 12s into the fixture
	startSec := float64(startIndex) * segmentSec
	outDir := filepath.Join(t.TempDir(), "job")
	if err := os.MkdirAll(filepath.Join(outDir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: input, StreamType: probe.StreamFile},
		OutputDir:  outDir,
		SegmentSec: segmentSec,
		MaxHeight:  720,
		Profile:    encode.DefaultHLSVideoProfile(720),
		Decode:     encode.VideoDecodeProfile{Codec: capabilities.CodecH264, Accel: capabilities.AccelNone},
		StartIndex: startIndex,
		StartSec:   startSec,
	}

	args, err := buildHLSContinuousArgs(runner, caps, opts)
	if err != nil {
		t.Fatalf("buildHLSContinuousArgs: %v", err)
	}
	// Output paths in the argv are relative to the job directory.
	cmd := exec.Command(ffmpegPath, args...)
	cmd.Dir = outDir
	if out, runErr := cmd.CombinedOutput(); runErr != nil {
		t.Fatalf("ffmpeg: %v\n%s", runErr, out)
	}

	video, audio := firstPacketTimes(t, ffprobePath, outDir)
	if math.Abs(video-startSec) > 0.5 {
		t.Fatalf("first video packet = %.3fs, want ~%.3fs", video, startSec)
	}
	if skew := math.Abs(video - audio); skew > 0.5 {
		t.Fatalf("audio/video skew = %.3fs, want < 0.5s (video %.3f, audio %.3f)", skew, video, audio)
	}
}

// firstPacketTimes reports the PTS of the first video and audio packet. The init
// segment carries the moov, so it is concatenated with the media segments to give
// ffprobe something it can parse.
func firstPacketTimes(t *testing.T, ffprobePath, outDir string) (video, audio float64) {
	t.Helper()
	segments, err := filepath.Glob(filepath.Join(outDir, "seg", "*.m4s"))
	if err != nil || len(segments) == 0 {
		t.Fatalf("no segments in %s: %v", outDir, err)
	}
	sort.Strings(segments)

	joined, err := os.ReadFile(filepath.Join(outDir, "init.m4s"))
	if err != nil {
		t.Fatalf("read init segment: %v", err)
	}
	for _, segment := range segments {
		data, readErr := os.ReadFile(segment)
		if readErr != nil {
			t.Fatalf("read %s: %v", segment, readErr)
		}
		joined = append(joined, data...)
	}
	probePath := filepath.Join(t.TempDir(), "joined.mp4")
	if err := os.WriteFile(probePath, joined, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(ffprobePath, "-v", "error",
		"-show_entries", "packet=codec_type,pts_time",
		"-of", "csv=p=0", probePath).CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe packets: %v\n%s", err, out)
	}

	video, audio = math.NaN(), math.NaN()
	for _, line := range strings.Split(string(out), "\n") {
		kind, value, ok := strings.Cut(strings.TrimSpace(line), ",")
		if !ok {
			continue
		}
		seconds, parseErr := strconv.ParseFloat(value, 64)
		if parseErr != nil {
			continue
		}
		if kind == "video" && math.IsNaN(video) {
			video = seconds
		}
		if kind == "audio" && math.IsNaN(audio) {
			audio = seconds
		}
		if !math.IsNaN(video) && !math.IsNaN(audio) {
			break
		}
	}
	if math.IsNaN(video) || math.IsNaN(audio) {
		t.Fatalf("missing video/audio packets (video=%v audio=%v)", video, audio)
	}
	return video, audio
}

func integrationFFmpegPaths(t *testing.T) (ffmpeg, ffprobe string) {
	t.Helper()
	ffmpeg = os.Getenv("GOFFMPEG_FFMPEG_PATH")
	ffprobe = os.Getenv("GOFFMPEG_FFPROBE_PATH")
	if ffmpeg == "" {
		var err error
		ffmpeg, err = exec.LookPath("ffmpeg")
		if err != nil {
			t.Fatalf("ffmpeg is required: %v", err)
		}
	}
	if ffprobe == "" {
		var err error
		ffprobe, err = exec.LookPath("ffprobe")
		if err != nil {
			t.Fatalf("ffprobe is required: %v", err)
		}
	}
	return ffmpeg, ffprobe
}

func generateIntegrationH264MKV(t *testing.T, ffmpegPath, outPath string, durationSec int) {
	t.Helper()
	cmd := exec.Command(ffmpegPath,
		"-hide_banner", "-y",
		"-f", "lavfi", "-i", "testsrc=duration="+strconv.Itoa(durationSec)+":size=1280x720:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440:duration="+strconv.Itoa(durationSec),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-g", "48", "-keyint_min", "48",
		"-preset", "veryfast", "-crf", "23",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate fixture: %v\n%s", err, out)
	}
}

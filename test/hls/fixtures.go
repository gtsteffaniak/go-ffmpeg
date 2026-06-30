package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

const defaultFixtureDurationSec = 120

// FixtureSpec describes one generated source file (2-minute sample).
type FixtureSpec struct {
	Name      string `json:"name"`
	Video     string `json:"video"`     // h264, hevc, vp9, av1
	Audio     string `json:"audio"`     // aac, mp3, ac3, eac3, opus, vorbis
	Container string `json:"container"` // mp4, mkv, mov, webm, avi
}

func allFixtureSpecs() []FixtureSpec {
	return []FixtureSpec{
		// H.264 — common containers and audio
		{Name: "h264_aac_mp4", Video: "h264", Audio: "aac", Container: "mp4"},
		{Name: "h264_aac_mkv", Video: "h264", Audio: "aac", Container: "mkv"},
		{Name: "h264_aac_mov", Video: "h264", Audio: "aac", Container: "mov"},
		{Name: "h264_mp3_mkv", Video: "h264", Audio: "mp3", Container: "mkv"},
		{Name: "h264_ac3_mkv", Video: "h264", Audio: "ac3", Container: "mkv"},
		{Name: "h264_eac3_mkv", Video: "h264", Audio: "eac3", Container: "mkv"},
		{Name: "h264_aac_avi", Video: "h264", Audio: "aac", Container: "avi"},
		{Name: "h264_mp3_avi", Video: "h264", Audio: "mp3", Container: "avi"},
		// HEVC
		{Name: "hevc_aac_mp4", Video: "hevc", Audio: "aac", Container: "mp4"},
		{Name: "hevc_aac_mkv", Video: "hevc", Audio: "aac", Container: "mkv"},
		{Name: "hevc_eac3_mkv", Video: "hevc", Audio: "eac3", Container: "mkv"},
		{Name: "hevc_ac3_mkv", Video: "hevc", Audio: "ac3", Container: "mkv"},
		{Name: "hevc_mp3_mkv", Video: "hevc", Audio: "mp3", Container: "mkv"},
		// VP9
		{Name: "vp9_opus_webm", Video: "vp9", Audio: "opus", Container: "webm"},
		{Name: "vp9_vorbis_webm", Video: "vp9", Audio: "vorbis", Container: "webm"},
		{Name: "vp9_aac_mkv", Video: "vp9", Audio: "aac", Container: "mkv"},
		// AV1
		{Name: "av1_opus_webm", Video: "av1", Audio: "opus", Container: "webm"},
		{Name: "av1_aac_mp4", Video: "av1", Audio: "aac", Container: "mp4"},
		{Name: "av1_aac_mkv", Video: "av1", Audio: "aac", Container: "mkv"},
		// MPEG-4 Part 2 legacy
		{Name: "mpeg4_aac_avi", Video: "mpeg4", Audio: "aac", Container: "avi"},
		{Name: "mpeg4_mp3_avi", Video: "mpeg4", Audio: "mp3", Container: "avi"},
	}
}

func resolveFixtureSpecs(namesFlag string) []FixtureSpec {
	all := allFixtureSpecs()
	if namesFlag == "" {
		return all
	}
	want := make(map[string]struct{})
	for _, name := range splitCSV(namesFlag) {
		want[name] = struct{}{}
	}
	var out []FixtureSpec
	for _, spec := range all {
		if _, ok := want[spec.Name]; ok {
			out = append(out, spec)
		}
	}
	return out
}

func fixtureFilename(spec FixtureSpec) string {
	ext := spec.Container
	if ext == "mkv" {
		ext = "mkv"
	}
	return spec.Name + "." + ext
}

func generateFixtures(ctx context.Context, reference, outDir string, durationSec int, specs []FixtureSpec) ([]FixtureResult, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	ffmpegPath, err := resolveFFmpegBin()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}
	if _, err := os.Stat(reference); err != nil {
		return nil, fmt.Errorf("reference video: %w", err)
	}

	var results []FixtureResult
	for _, spec := range specs {
		outPath := filepath.Join(outDir, fixtureFilename(spec))
		fr := FixtureResult{Spec: spec, Path: outPath}
		if st, err := os.Stat(outPath); err == nil && st.Size() > 0 {
			if dur, err := probeFixtureDurationSec(outPath); err == nil && durationSec > 0 {
				if math.Abs(dur-float64(durationSec)) <= 1.0 {
					fr.Generated = true
					fr.Skipped = true
					fr.Message = "already exists"
					results = append(results, fr)
					continue
				}
				fr.Message = fmt.Sprintf("regenerating (was %.1fs, want %ds)", dur, durationSec)
			} else {
				fr.Generated = true
				fr.Skipped = true
				fr.Message = "already exists"
				results = append(results, fr)
				continue
			}
		}
		start := time.Now()
		err := transcodeReferenceToFixture(ctx, ffmpegPath, reference, outPath, spec, durationSec)
		fr.GenerateMs = time.Since(start).Milliseconds()
		if err != nil {
			fr.Error = err.Error()
			results = append(results, fr)
			continue
		}
		fr.Generated = true
		results = append(results, fr)
	}
	return results, nil
}

func transcodeReferenceToFixture(ctx context.Context, ffmpeg, reference, outPath string, spec FixtureSpec, durationSec int) error {
	vEnc, vExtra, err := videoEncoderArgs(spec.Video)
	if err != nil {
		return err
	}
	aEnc, err := audioEncoderArgs(spec.Audio)
	if err != nil {
		return err
	}
	args := []string{
		"-hide_banner", "-y",
		"-i", reference,
		"-t", fmt.Sprintf("%d", durationSec),
		"-map", "0:v:0?",
		"-map", "0:a:0?",
	}
	args = append(args, vEnc...)
	args = append(args, vExtra...)
	args = append(args, aEnc...)
	if spec.Container == "webm" {
		args = append(args, "-f", "webm")
	}
	if spec.Container == "avi" {
		args = append(args, "-f", "avi")
	}
	args = append(args, outPath)

	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := parseFFmpegError([]byte(stderr.String()))
		if msg == "" {
			msg = stderr.String()
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}

func probeFixtureDurationSec(path string) (float64, error) {
	ffprobePath, err := resolveFFprobeBin()
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(ffprobePath, "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func videoEncoderArgs(codec string) (enc []string, extra []string, err error) {
	switch codec {
	case "h264":
		return []string{"-c:v", "libx264", "-pix_fmt", "yuv420p", "-g", "30", "-keyint_min", "30"},
			[]string{"-preset", "veryfast", "-crf", "23"}, nil
	case "hevc":
		return []string{"-c:v", "libx265", "-pix_fmt", "yuv420p", "-g", "30"},
			[]string{"-preset", "veryfast", "-crf", "28"}, nil
	case "vp9":
		return []string{"-c:v", "libvpx-vp9", "-pix_fmt", "yuv420p", "-g", "30"},
			[]string{"-b:v", "1M", "-row-mt", "1"}, nil
	case "av1":
		return []string{"-c:v", "libsvtav1", "-pix_fmt", "yuv420p", "-g", "30"},
			[]string{"-preset", "8", "-crf", "35"}, nil
	case "mpeg4":
		return []string{"-c:v", "mpeg4", "-pix_fmt", "yuv420p", "-g", "30"},
			[]string{"-q:v", "5"}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported video codec %q", codec)
	}
}

func audioEncoderArgs(codec string) ([]string, error) {
	switch codec {
	case "aac":
		return []string{"-c:a", "aac", "-b:a", "128k"}, nil
	case "mp3":
		return []string{"-c:a", "libmp3lame", "-b:a", "128k"}, nil
	case "ac3":
		return []string{"-c:a", "ac3", "-b:a", "192k"}, nil
	case "eac3":
		return []string{"-c:a", "eac3", "-b:a", "192k"}, nil
	case "opus":
		return []string{"-c:a", "libopus", "-b:a", "96k"}, nil
	case "vorbis":
		return []string{"-c:a", "libvorbis", "-b:a", "128k"}, nil
	default:
		return nil, fmt.Errorf("unsupported audio codec %q", codec)
	}
}

func canRemuxFixture(info goffmpeg.StreamInfo) bool {
	return goffmpeg.CanFMP4StreamCopy(info)
}

func canCopyFixture(info goffmpeg.StreamInfo) bool {
	return goffmpeg.CanH264VideoCopy(info)
}

type FixtureResult struct {
	Spec       FixtureSpec `json:"spec"`
	Path       string      `json:"path"`
	Generated  bool        `json:"generated"`
	Skipped    bool        `json:"skipped,omitempty"`
	GenerateMs int64       `json:"generateMs,omitempty"`
	Error      string      `json:"error,omitempty"`
	Message    string      `json:"message,omitempty"`
}

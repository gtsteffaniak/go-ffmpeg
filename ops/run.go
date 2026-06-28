package ops

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// InputSource describes an ffmpeg input.
type InputSource struct {
	URL        string
	StreamType probe.StreamType
}

// RTSPInputOptions adds RTSP-specific input flags.
type RTSPInputOptions struct {
	Transport       string
	AnalyzeDuration string
	ProbeSize       string
}

// ScreenshotOptions configures frame extraction to a file.
type ScreenshotOptions struct {
	Input      InputSource
	OutputPath string
	Quality    int
	RTSP       *RTSPInputOptions
	Timeout    time.Duration
}

// Screenshot extracts a single frame to OutputPath.
func Screenshot(ctx context.Context, runner *ffexec.Runner, opts ScreenshotOptions) error {
	args := []string{"-hide_banner", "-nostats"}
	args = appendInputFlags(args, opts.Input, opts.RTSP, nil)
	args = append(args,
		"-i", opts.Input.URL,
		"-frames:v", "1",
		"-update", "1",
		"-q:v", encode.QualityToQScale(opts.Quality),
		"-f", "image2",
		"-loglevel", "error",
		opts.OutputPath,
	)
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := runner.RunFFmpeg(pctx, args...)
	return err
}

// PreviewOptions configures MJPEG preview generation.
type PreviewOptions struct {
	Input       string
	SeekPercent float64
	Width       int
	Height      int
	Quality     int
	Timeout     time.Duration
}

// VideoPreview extracts an MJPEG frame to w.
func VideoPreview(ctx context.Context, runner *ffexec.Runner, w io.Writer, opts PreviewOptions) error {
	dur, err := probe.GetMediaDuration(ctx, runner, opts.Input)
	if err != nil {
		return err
	}
	seekPct := opts.SeekPercent
	if seekPct <= 0 || seekPct > 100 {
		seekPct = 10
	}
	seekSec := dur * seekPct / 100

	args := []string{"-hide_banner", "-nostats", "-ss", fmt.Sprintf("%.3f", seekSec), "-i", opts.Input}
	if opts.Width > 0 && opts.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", opts.Width, opts.Height))
	}
	q := opts.Quality
	if q == 0 {
		q = 10
	}
	args = append(args,
		"-frames:v", "1",
		"-q:v", encode.QualityToQScale(q),
		"-vcodec", "mjpeg",
		"-f", "image2",
		"-",
	)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := ffexec.CommandContext(pctx, runner.FFmpegPath, args...)
	cmd.Stdout = w
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if pctx.Err() != nil {
			return pctx.Err()
		}
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

// TranscodeOptions configures file/URL transcoding.
type TranscodeOptions struct {
	Input      InputSource
	OutputPath string
	Profile    encode.VideoProfile
	AudioCodec string
	RTSP       *RTSPInputOptions
	Timeout    time.Duration
	ExtraArgs  []string
}

// Transcode re-encodes input to OutputPath.
func Transcode(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts TranscodeOptions) error {
	resolver := encode.NewResolver(caps)
	vidArgs, err := resolver.VideoEncoderArgs(opts.Profile)
	if err != nil {
		return err
	}
	args := []string{"-hide_banner", "-nostats"}
	args = appendInputFlags(args, opts.Input, opts.RTSP, nil)
	args = append(args, "-i", opts.Input.URL)
	args = append(args, vidArgs...)
	audio := opts.AudioCodec
	if audio == "" {
		audio = "aac"
	}
	args = append(args, "-c:a", audio, "-y", opts.OutputPath)
	args = append(args, opts.ExtraArgs...)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err = runner.RunFFmpeg(pctx, args...)
	return err
}

// SegmentRecordOptions configures segmented recording.
type SegmentRecordOptions struct {
	Input           InputSource
	OutputDir       string
	SegmentPattern  string
	SegmentListPath string
	SegmentDuration time.Duration
	Profile         encode.VideoProfile
	RTSP            *RTSPInputOptions
	ExtraInputArgs  []string
	ExtraOutputArgs []string
}

// SegmentRecord starts segmented recording; returns after ffmpeg completes or ctx cancelled.
func SegmentRecord(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts SegmentRecordOptions) error {
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return err
	}
	resolver := encode.NewResolver(caps)
	vidArgs, err := resolver.VideoEncoderArgs(opts.Profile)
	if err != nil {
		return err
	}
	segDur := opts.SegmentDuration
	if segDur == 0 {
		segDur = time.Hour
	}
	pattern := opts.SegmentPattern
	if pattern == "" {
		pattern = "%Y-%m-%d_%Hh%Mm_seg.mp4"
	}
	listPath := opts.SegmentListPath
	if listPath == "" {
		listPath = filepath.Join(opts.OutputDir, "segments.ffcat")
	}

	args := []string{"-hide_banner", "-nostats", "-fflags", "+genpts", "-correct_ts_overflow", "1"}
	args = appendInputFlags(args, opts.Input, opts.RTSP, nil)
	args = append(args, opts.ExtraInputArgs...)
	args = append(args, "-i", opts.Input.URL)
	args = append(args, vidArgs...)
	args = append(args,
		"-c:a", "aac",
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%d", int(segDur.Seconds())),
		"-segment_format", "mp4",
		"-strftime", "1",
		"-segment_list", listPath,
		"-segment_list_type", "ffconcat",
		"-reset_timestamps", "1",
	)
	args = append(args, opts.ExtraOutputArgs...)
	args = append(args, filepath.Join(opts.OutputDir, pattern))

	_, err = runner.RunFFmpeg(ctx, args...)
	return err
}

// FMP4StreamCopyOptions configures live fMP4 stream copy to a writer.
type FMP4StreamCopyOptions struct {
	Input InputSource
	RTSP  *RTSPInputOptions
}

// FMP4TranscodeOptions configures live fMP4 transcoding to a writer.
type FMP4TranscodeOptions struct {
	Input      InputSource
	RTSP       *RTSPInputOptions
	Decode     encode.VideoDecodeProfile
	Profile    encode.VideoProfile
	AudioCodec string
	MaxHeight  int // max output height in pixels; 0 disables downscale
}

// FMP4Transcode re-encodes input to fragmented MP4 on w until ctx cancelled.
func FMP4Transcode(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, w io.Writer, opts FMP4TranscodeOptions) error {
	resolver := encode.NewResolver(caps)
	decodeArgs, err := resolver.VideoDecoderArgs(opts.Decode)
	if err != nil {
		return err
	}
	vidArgs, err := resolver.VideoEncoderArgs(opts.Profile)
	if err != nil {
		return err
	}

	args := []string{"-hide_banner", "-nostats"}
	args = appendInputFlags(args, opts.Input, opts.RTSP, nil)
	args = append(args, decodeArgs...)
	args = append(args, "-i", opts.Input.URL)
	if opts.MaxHeight > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=-2:min(%d\\,ih)", opts.MaxHeight))
	}
	args = append(args, vidArgs...)
	// Browser MSE expects baseline H.264 (avc1.42E01E); libx264 defaults to High.
	if opts.Profile.Codec == "" || opts.Profile.Codec == encode.CodecH264 {
		args = append(args, "-profile:v", "baseline", "-level", "3.1", "-tag:v", "avc1")
	}
	audio := opts.AudioCodec
	if audio == "" {
		audio = "aac"
	}
	args = append(args,
		"-c:a", audio,
		"-fflags", "+igndts",
		"-avoid_negative_ts", "make_zero",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-frag_duration", "1000000",
		"-f", "mp4",
		"-loglevel", runner.FFmpegLogLevel(),
		"-",
	)
	return runFMP4ToWriter(ctx, runner, w, args)
}

// FMP4StreamCopy copies streams to fragmented MP4 on w until ctx cancelled.
func FMP4StreamCopy(ctx context.Context, runner *ffexec.Runner, w io.Writer, opts FMP4StreamCopyOptions) error {
	args := []string{"-hide_banner", "-nostats"}
	args = appendInputFlags(args, opts.Input, opts.RTSP, nil)
	args = append(args,
		"-i", opts.Input.URL,
		"-c:v", "copy",
		"-c:a", "copy",
		"-fflags", "+igndts",
		"-avoid_negative_ts", "make_zero",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-frag_duration", "1000000",
		"-f", "mp4",
		"-loglevel", runner.FFmpegLogLevel(),
		"-",
	)
	return runFMP4ToWriter(ctx, runner, w, args)
}

func runFMP4ToWriter(ctx context.Context, runner *ffexec.Runner, w io.Writer, args []string) error {
	if runner != nil && runner.VerboseFFmpeg {
		fmt.Fprintf(os.Stderr, "[ffmpeg] %s %s\n", runner.FFmpegPath, strings.Join(args, " "))
	}
	cmd := ffexec.CommandContext(ctx, runner.FFmpegPath, args...)
	cmd.Stdout = w
	var stderr strings.Builder
	cmd.Stderr = runner.FFmpegStderrWriter(&stderr)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return ctx.Err()
	case err := <-done:
		if err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil && stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, stderr.String())
		}
		return err
	}
}

// TimelapseCompileOptions compiles images into a video.
type TimelapseCompileOptions struct {
	ConcatListPath string
	OutputPath     string
	FPS            float64
	Width          int
	Height         int
	Profile        encode.VideoProfile
}

// TimelapseCompile builds a video from a concat demuxer list file.
func TimelapseCompile(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts TimelapseCompileOptions) error {
	resolver := encode.NewResolver(caps)
	vidArgs, err := resolver.VideoEncoderArgs(opts.Profile)
	if err != nil {
		return err
	}
	fps := opts.FPS
	if fps <= 0 {
		fps = 24
	}
	args := []string{
		"-hide_banner", "-nostats",
		"-f", "concat", "-safe", "0",
		"-i", opts.ConcatListPath,
	}
	if opts.Width > 0 && opts.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", opts.Width, opts.Height))
	}
	args = append(args, "-r", fmt.Sprintf("%.1f", fps))
	args = append(args, vidArgs...)
	args = append(args, "-pix_fmt", "yuv420p", "-y", opts.OutputPath)
	_, err = runner.RunFFmpeg(ctx, args...)
	return err
}

// ConvertHEICOptions configures HEIC to JPEG conversion.
type ConvertHEICOptions struct {
	InputPath         string
	OutputPath        string
	Width             int
	Height            int
	Quality           int
	OrientationFilter string
}

// ConvertHEIC converts HEIC/HEIF to JPEG.
func ConvertHEIC(ctx context.Context, runner *ffexec.Runner, opts ConvertHEICOptions) error {
	args := []string{"-hide_banner", "-nostats", "-i", opts.InputPath}
	if opts.OrientationFilter != "" {
		args = append(args, "-vf", opts.OrientationFilter)
	}
	if opts.Width > 0 && opts.Height > 0 {
		if opts.OrientationFilter != "" {
			args[len(args)-1] = opts.OrientationFilter + fmt.Sprintf(",scale=%d:%d:force_original_aspect_ratio=decrease", opts.Width, opts.Height)
		} else {
			args = append(args, "-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", opts.Width, opts.Height))
		}
	}
	q := opts.Quality
	if q == 0 {
		q = 5
	}
	args = append(args,
		"-q:v", encode.QualityToQScale(q),
		"-pix_fmt", "yuvj420p",
		"-y", opts.OutputPath,
	)
	_, err := runner.RunFFmpeg(ctx, args...)
	return err
}

func appendInputFlags(args []string, input InputSource, rtsp *RTSPInputOptions, extra *InputExtraFlags) []string {
	switch strings.ToLower(string(input.StreamType)) {
	case "rtsp":
		transport := "tcp"
		analyze := "10000000"
		probeSize := "50000000"
		if rtsp != nil {
			if rtsp.Transport != "" {
				transport = rtsp.Transport
			}
			if rtsp.AnalyzeDuration != "" {
				analyze = rtsp.AnalyzeDuration
			}
			if rtsp.ProbeSize != "" {
				probeSize = rtsp.ProbeSize
			}
		}
		args = append(args, "-rtsp_transport", transport, "-analyzeduration", analyze, "-probesize", probeSize)
	}
	if extra != nil && extra.Throttle != nil && extra.Throttle.Enabled {
		args = encode.AppendReadrateArgs(args, extra.Features.Version, *extra.Throttle)
	}
	return args
}

// InputExtraFlags optional pre-input ffmpeg flags (e.g. readrate throttling).
type InputExtraFlags struct {
	Throttle *encode.ThrottleConfig
	Features capabilities.FeatureFlags
}

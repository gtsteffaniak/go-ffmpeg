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
	// SeekSec seeks to this timestamp (seconds). When zero, SeekPercent or the first frame is used.
	SeekSec float64
	// SeekPercent seeks to this percentage of duration when SeekSec is zero.
	SeekPercent float64
	// DurationSec when > 0 skips ffprobe for seek-percent and enables lightweight input probing.
	DurationSec float64
	// LightProbe overrides default lightweight -probesize/-analyzeduration when duration/seek is known.
	LightProbe *CaptureLightProbe
	Width      int
	Height     int
	ScaleMode  ScaleMode
	// Accuracy defaults to CaptureFast (nearest keyframe).
	Accuracy CaptureAccuracy
}

// Screenshot extracts a single frame to OutputPath via image2pipe (no ffmpeg temp files).
//
// Deprecated: use CaptureFrame/CaptureFrameFile.
func Screenshot(ctx context.Context, runner *ffexec.Runner, opts ScreenshotOptions) error {
	dur := opts.DurationSec
	if dur <= 0 && opts.SeekPercent > 0 && opts.SeekSec <= 0 {
		var err error
		dur, err = probe.GetMediaDuration(ctx, runner, opts.Input.URL)
		if err != nil {
			return err
		}
		opts.DurationSec = dur
	}
	fo := ScreenshotOptionsToFrame(opts)
	if fo.Timeout == 0 {
		fo.Timeout = 30 * time.Second
	}
	seekSec := ResolveCaptureSeekSec(opts.DurationSec, opts.SeekSec, opts.SeekPercent)
	return captureFrameToPath(ctx, runner, nil, opts.OutputPath, fo, seekSec, false)
}

// ScaleMode controls how preview frames are scaled to target dimensions.
type ScaleMode int

const (
	// ScaleFit scales down to fit within width×height without upscaling (default).
	ScaleFit ScaleMode = iota
	// ScaleFill scales up and center-crops to fill width×height.
	ScaleFill
)

// PreviewScaleFilter returns the -vf filter for sized preview output.
func PreviewScaleFilter(width, height int, mode ScaleMode) string {
	switch mode {
	case ScaleFill:
		return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", width, height, width, height)
	default:
		// Cap at input dimensions so small sources are never upscaled.
		return fmt.Sprintf("scale=min(%d\\,iw):min(%d\\,ih):force_original_aspect_ratio=decrease", width, height)
	}
}

// PreviewOptions configures MJPEG preview generation.
type PreviewOptions struct {
	Input       string
	SeekPercent float64
	SeekSec     float64 // when > 0, seek to this timestamp instead of SeekPercent
	// DurationSec when > 0 skips the duration ffprobe (caller already probed).
	DurationSec float64
	Width       int
	Height      int
	ScaleMode   ScaleMode
	Quality     int
	Timeout     time.Duration
	// LightProbe overrides default lightweight -probesize/-analyzeduration when duration/seek is known.
	LightProbe *CaptureLightProbe
	// Accuracy defaults to CaptureFast (nearest keyframe).
	Accuracy CaptureAccuracy
}

// ResolvePreviewSeekSec computes the seek position from duration and options.
func ResolvePreviewSeekSec(dur float64, opts PreviewOptions) float64 {
	if opts.SeekSec > 0 {
		seekSec := opts.SeekSec
		if dur > 0 && seekSec > dur {
			seekSec = dur
		}
		return seekSec
	}
	seekPct := opts.SeekPercent
	if seekPct <= 0 || seekPct > 100 {
		seekPct = 10
	}
	return dur * seekPct / 100
}

// VideoPreview probes duration then extracts an MJPEG frame to w.
// Prefer Service.VideoPreview, which uses separate probe and decode concurrency slots.
//
// Deprecated: use CaptureFrame/CaptureFrameFile.
func VideoPreview(ctx context.Context, runner *ffexec.Runner, w io.Writer, opts PreviewOptions) error {
	dur := opts.DurationSec
	if PreviewNeedsDurationProbe(opts) {
		var err error
		dur, err = probe.GetMediaDuration(ctx, runner, opts.Input)
		if err != nil {
			return err
		}
	}
	return VideoPreviewFrame(ctx, runner, w, opts, ResolvePreviewSeekSec(dur, opts))
}

// VideoPreviewFrame extracts an MJPEG frame to w at seekSec without probing duration.
//
// Deprecated: use CaptureFrame.
func VideoPreviewFrame(ctx context.Context, runner *ffexec.Runner, w io.Writer, opts PreviewOptions, seekSec float64) error {
	fo := PreviewOptionsToFrame(opts, seekSec)
	return CaptureFrame(ctx, runner, nil, w, fo, seekSec)
}

// TranscodeOptions configures file/URL transcoding.
type TranscodeOptions struct {
	Input      InputSource
	OutputPath string
	Profile    encode.VideoProfile
	AudioCodec string
	RTSP       *RTSPInputOptions
	Timeout    time.Duration
	// ExtraArgs is deprecated: use ExtraOutputArgs (and ExtraInputArgs when needed).
	// When ExtraInputArgs and ExtraOutputArgs are both empty, ExtraArgs is applied
	// as ExtraOutputArgs before the output path (legacy callers relied on output-side flags).
	ExtraArgs       []string
	ExtraInputArgs  []string
	ExtraOutputArgs []string
}

// BuildTranscodeArgs assembles ffmpeg argv for Transcode (exported for tests).
func BuildTranscodeArgs(runner *ffexec.Runner, caps *capabilities.Capabilities, opts TranscodeOptions) ([]string, error) {
	resolver := encode.NewResolver(caps)
	vidArgs, err := resolver.VideoEncoderArgs(opts.Profile)
	if err != nil {
		return nil, err
	}
	extraIn := opts.ExtraInputArgs
	extraOut := opts.ExtraOutputArgs
	if len(opts.ExtraArgs) > 0 {
		if len(extraIn) == 0 && len(extraOut) == 0 {
			extraOut = opts.ExtraArgs
		} else {
			extraOut = append(append([]string(nil), extraOut...), opts.ExtraArgs...)
		}
	}
	args := []string{"-hide_banner", "-nostats"}
	args = appendInputFlags(args, runner, opts.Input, opts.RTSP, nil)
	args = append(args, extraIn...)
	args = append(args, "-i", opts.Input.URL)
	args = append(args, vidArgs...)
	audio := opts.AudioCodec
	if audio == "" {
		audio = "aac"
	}
	args = append(args, "-c:a", audio)
	args = append(args, extraOut...)
	args = append(args, "-y", opts.OutputPath)
	return args, nil
}

// Transcode re-encodes input to OutputPath.
func Transcode(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts TranscodeOptions) error {
	args, err := BuildTranscodeArgs(runner, caps, opts)
	if err != nil {
		return err
	}

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
	args = appendInputFlags(args, runner, opts.Input, opts.RTSP, nil)
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
	args = appendInputFlags(args, runner, opts.Input, opts.RTSP, nil)
	args = append(args, decodeArgs...)
	args = append(args, "-i", opts.Input.URL)
	if opts.MaxHeight > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=-2:min(%d\\,ih)", opts.MaxHeight))
	}
	args = append(args, vidArgs...)
	if opts.Profile.Codec == "" || opts.Profile.Codec == encode.CodecH264 {
		if encSel, err := resolver.ResolveEncoder(opts.Profile); err == nil {
			args = encode.AppendH264FMP4CompatArgs(args, encSel.Accel, opts.MaxHeight)
		}
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
	args = appendInputFlags(args, runner, opts.Input, opts.RTSP, nil)
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
	args := []string{"-hide_banner", "-nostats"}
	args = runner.AppendProtocolWhitelistArgs(args)
	args = append(args,
		"-f", "concat", "-safe", "0",
		"-i", opts.ConcatListPath,
	)
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
	args := []string{"-hide_banner", "-nostats"}
	args = runner.AppendProtocolWhitelistArgs(args)
	args = append(args, "-i", opts.InputPath)
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

func appendInputFlags(args []string, runner *ffexec.Runner, input InputSource, rtsp *RTSPInputOptions, extra *InputExtraFlags) []string {
	args = runner.AppendProtocolWhitelistArgs(args)
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

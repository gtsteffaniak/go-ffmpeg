package ops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
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

// FrameFormat is the still-image output codec/container.
type FrameFormat int

const (
	// FrameJPEG is Motion JPEG over image2pipe (default).
	FrameJPEG FrameFormat = 0
	// FrameWebP uses libwebp or the native webp encoder when available.
	FrameWebP FrameFormat = 1
	// FramePNG uses the png encoder.
	FramePNG FrameFormat = 2
)

// FramePosition selects which frame to capture. The zero value is the first frame.
type FramePosition struct {
	kind  framePositionKind
	value float64
}

type framePositionKind int

const (
	framePosFirst   framePositionKind = 0
	framePosSeconds framePositionKind = 1
	framePosPercent framePositionKind = 2
)

// AtSec seeks to the given timestamp in seconds on the media timeline.
func AtSec(sec float64) FramePosition {
	return FramePosition{kind: framePosSeconds, value: sec}
}

// AtPercent seeks to a percentage of the media duration (0–100).
func AtPercent(percent float64) FramePosition {
	return FramePosition{kind: framePosPercent, value: percent}
}

// FrameOptions configures single-frame capture to a writer or file.
type FrameOptions struct {
	// Input is the media source (URL/path and stream type).
	Input InputSource
	// RTSP carries transport and probe tuning for RTSP inputs.
	RTSP *RTSPInputOptions
	// At selects which frame to capture (zero value = first frame).
	At FramePosition
	// DurationSec skips ffprobe when the caller already knows media length.
	DurationSec float64
	// Accuracy selects fast keyframe seek vs exact decode-to-time.
	Accuracy CaptureAccuracy
	// Width and Height scale the output when both are positive.
	Width  int
	Height int
	// ScaleMode controls letterboxing when scaling.
	ScaleMode ScaleMode
	// Format selects JPEG, WebP, or PNG output.
	Format FrameFormat
	// Quality is 1–100 (higher is better); 0 uses encode.DefaultFrameQuality. PNG ignores quality.
	Quality int
	// LightProbe sets lightweight ffmpeg input -probesize/-analyzeduration when duration/seek is known.
	LightProbe *CaptureLightProbe
	// MeanLuma when non-nil is filled with average luma (0–255) of the encoded frame.
	MeanLuma *float64
	// Timeout bounds the ffmpeg invocation (default 60s).
	Timeout time.Duration
}

// FrameCaptureValidationError reports invalid FrameOptions before ffmpeg runs.
type FrameCaptureValidationError struct {
	Field string
	Msg   string
}

func (e *FrameCaptureValidationError) Error() string {
	if e.Field == "" {
		return "frame capture: " + e.Msg
	}
	return fmt.Sprintf("frame capture: %s: %s", e.Field, e.Msg)
}

var errFrameFormatUnsupported = errors.New("frame capture: image format encoder unavailable")

// FrameNeedsDurationProbe reports whether ffprobe duration is required for position resolution.
func FrameNeedsDurationProbe(at FramePosition, durationSec float64) bool {
	return at.kind == framePosPercent && durationSec <= 0
}

// PreviewNeedsDurationProbe reports whether VideoPreview must probe duration (legacy API).
func PreviewNeedsDurationProbe(opts PreviewOptions) bool {
	if opts.DurationSec > 0 {
		return false
	}
	if opts.SeekSec > 0 {
		return false
	}
	return true
}

// ValidateFramePosition checks At for negative values.
func ValidateFramePosition(at FramePosition) error {
	switch at.kind {
	case framePosSeconds:
		if at.value < 0 {
			return &FrameCaptureValidationError{Field: "At", Msg: "seconds must be non-negative"}
		}
	case framePosPercent:
		if at.value < 0 {
			return &FrameCaptureValidationError{Field: "At", Msg: "percent must be non-negative"}
		}
	}
	return nil
}

// ValidateFrameOptions checks opts before capture. caps may be nil to skip encoder gates.
func ValidateFrameOptions(caps *capabilities.Capabilities, opts FrameOptions) error {
	if strings.TrimSpace(opts.Input.URL) == "" {
		return &FrameCaptureValidationError{Field: "Input.URL", Msg: "required"}
	}
	if err := ValidateFramePosition(opts.At); err != nil {
		return err
	}
	if opts.Width < 0 || opts.Height < 0 {
		return &FrameCaptureValidationError{Field: "Width/Height", Msg: "must be non-negative"}
	}
	if opts.Width > 0 && opts.Height == 0 || opts.Height > 0 && opts.Width == 0 {
		return &FrameCaptureValidationError{Field: "Width/Height", Msg: "both dimensions required when scaling"}
	}
	if _, err := resolveFrameEncoder(caps, opts.Format); err != nil {
		if errors.Is(err, errFrameFormatUnsupported) {
			return &FrameCaptureValidationError{Field: "Format", Msg: "encoder unavailable in detected ffmpeg build"}
		}
		return err
	}
	return nil
}

// ResolveFrameSeekSec maps position and duration to a seek time in seconds.
// Percent values above 100 are clamped to 100. Seconds beyond duration clamp to a safe point near the end.
func ResolveFrameSeekSec(at FramePosition, durationSec float64) float64 {
	switch at.kind {
	case framePosSeconds:
		sec := at.value
		if durationSec > 0 && sec > durationSec {
			return clampSeekNearEnd(durationSec)
		}
		return sec
	case framePosPercent:
		pct := at.value
		if pct > 100 {
			pct = 100
		}
		if durationSec <= 0 {
			return 0
		}
		return durationSec * pct / 100
	default:
		return 0
	}
}

func clampSeekNearEnd(durationSec float64) float64 {
	if durationSec <= 0 {
		return 0
	}
	const epsilon = 0.05
	if durationSec > epsilon {
		return durationSec - epsilon
	}
	return 0
}

type frameEncoder struct {
	vcodec string
	muxer  string
}

func resolveFrameEncoder(caps *capabilities.Capabilities, format FrameFormat) (frameEncoder, error) {
	switch format {
	case FrameWebP:
		if caps != nil {
			if caps.EncoderAvailable("libwebp") {
				return frameEncoder{vcodec: "libwebp", muxer: "webp"}, nil
			}
			if caps.EncoderAvailable("webp") {
				return frameEncoder{vcodec: "webp", muxer: "webp"}, nil
			}
			return frameEncoder{}, errFrameFormatUnsupported
		}
		return frameEncoder{vcodec: "libwebp", muxer: "webp"}, nil
	case FramePNG:
		if caps != nil && !caps.EncoderAvailable("png") {
			return frameEncoder{}, errFrameFormatUnsupported
		}
		return frameEncoder{vcodec: "png", muxer: "image2pipe"}, nil
	default:
		if caps != nil && !caps.EncoderAvailable("mjpeg") {
			return frameEncoder{}, errFrameFormatUnsupported
		}
		return frameEncoder{vcodec: "mjpeg", muxer: "image2pipe"}, nil
	}
}

func frameCaptureArgs(opts FrameOptions, enc frameEncoder, seekSec float64) FrameCaptureArgs {
	scale := ""
	if opts.Width > 0 && opts.Height > 0 {
		scale = PreviewScaleFilter(opts.Width, opts.Height, opts.ScaleMode)
	}
	skipProbe := opts.DurationSec > 0 || seekSec > 0 || opts.At.kind != framePosFirst
	fa := FrameCaptureArgs{
		SeekSec:      seekSec,
		Accuracy:     opts.Accuracy,
		SkipProbe:    skipProbe,
		LightProbe:   opts.LightProbe,
		KeyframeOnly: opts.Accuracy == CaptureFast,
		ScaleFilter:  scale,
		VideoCodec:   enc.vcodec,
		MuxerFormat:  enc.muxer,
	}
	switch opts.Format {
	case FrameWebP:
		fa.WebPQuality = encode.WebPQuality(opts.Quality)
	default:
		fa.QScale = encode.MJPEGQualityQScale(opts.Quality)
	}
	return fa
}

// CaptureFrame streams one encoded frame to w at seekSec (from ResolveFrameSeekSec).
func CaptureFrame(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, w io.Writer, opts FrameOptions, seekSec float64) error {
	if err := ValidateFrameOptions(caps, opts); err != nil {
		return err
	}
	enc, err := resolveFrameEncoder(caps, opts.Format)
	if err != nil {
		return err
	}

	dest := w
	var buf *bytes.Buffer
	if opts.MeanLuma != nil {
		buf = &bytes.Buffer{}
		dest = buf
	}

	args := BuildFrameCaptureArgs(runner, opts.Input, opts.RTSP, frameCaptureArgs(opts, enc, seekSec))

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := runFrameCaptureToWriter(pctx, runner, args, dest); err != nil {
		return err
	}

	if buf != nil {
		if opts.MeanLuma != nil {
			if luma, err := meanLumaFromImage(buf.Bytes()); err == nil {
				*opts.MeanLuma = luma
			}
		}
		if _, err := io.Copy(w, buf); err != nil {
			return err
		}
	}
	return nil
}

// CaptureFrameFile writes one frame to path using a temp file in the same directory and atomic rename.
func CaptureFrameFile(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, path string, opts FrameOptions, seekSec float64) error {
	return captureFrameToPath(ctx, runner, caps, path, opts, seekSec, true)
}

func captureFrameToPath(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, path string, opts FrameOptions, seekSec float64, atomic bool) error {
	if strings.TrimSpace(path) == "" {
		return &FrameCaptureValidationError{Field: "path", Msg: "required"}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	if atomic {
		tmp, err := os.CreateTemp(dir, ".goffmpeg-capture-*")
		if err != nil {
			return err
		}
		tmpPath := tmp.Name()
		cleanup := true
		defer func() {
			_ = tmp.Close()
			if cleanup {
				_ = os.Remove(tmpPath)
			}
		}()
		if err := CaptureFrame(ctx, runner, caps, tmp, opts, seekSec); err != nil {
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		if err := os.Chmod(tmpPath, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, path); err != nil {
			return err
		}
		cleanup = false
		return nil
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := CaptureFrame(ctx, runner, caps, f, opts, seekSec); err != nil {
		_ = os.Remove(path)
		return err
	}
	return f.Sync()
}

func meanLumaFromImage(data []byte) (float64, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	bounds := img.Bounds()
	var sum float64
	var n int
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			luma := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
			sum += luma
			n++
		}
	}
	if n == 0 {
		return 0, fmt.Errorf("empty image")
	}
	return sum / float64(n), nil
}

// ScreenshotOptionsToFrame maps legacy screenshot options to FrameOptions.
func ScreenshotOptionsToFrame(opts ScreenshotOptions) FrameOptions {
	q := opts.Quality
	if q == 0 {
		q = 85
	}
	fo := FrameOptions{
		Input:       opts.Input,
		RTSP:        opts.RTSP,
		Quality:     q,
		Timeout:     opts.Timeout,
		Format:      FrameJPEG,
		Width:       opts.Width,
		Height:      opts.Height,
		ScaleMode:   opts.ScaleMode,
		Accuracy:    opts.Accuracy,
		LightProbe:  opts.LightProbe,
		DurationSec: opts.DurationSec,
	}
	if opts.SeekSec > 0 {
		fo.At = AtSec(opts.SeekSec)
	} else if opts.SeekPercent > 0 {
		fo.At = AtPercent(opts.SeekPercent)
	}
	return fo
}

// PreviewOptionsToFrame maps legacy preview options to FrameOptions (MJPEG stream).
func PreviewOptionsToFrame(opts PreviewOptions, seekSec float64) FrameOptions {
	q := opts.Quality
	if q == 0 {
		q = 10
	}
	return FrameOptions{
		Input:       InputSource{URL: opts.Input, StreamType: probe.StreamFile},
		At:          AtSec(seekSec),
		DurationSec: opts.DurationSec,
		Width:       opts.Width,
		Height:      opts.Height,
		ScaleMode:   opts.ScaleMode,
		Quality:     q,
		Timeout:     opts.Timeout,
		Accuracy:    opts.Accuracy,
		LightProbe:  opts.LightProbe,
		Format:      FrameJPEG,
	}
}

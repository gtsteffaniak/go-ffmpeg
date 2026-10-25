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

// CaptureAccuracy selects how ffmpeg seeks before extracting one frame.
type CaptureAccuracy int

const (
	// CaptureFast seeks before demux (-ss before -i). Default.
	CaptureFast CaptureAccuracy = 0
	// CaptureExact decodes from the start and seeks after -i for frame-accurate output.
	CaptureExact CaptureAccuracy = 1
)

// FrameFormat is the still-image output codec/container.
type FrameFormat int

const (
	// FrameJPEG is Motion JPEG in an image2 bitstream (default).
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

// CaptureLightProbe optionally records mean luma (0–255) of the captured frame.
type CaptureLightProbe struct {
	MeanLuma float64
}

// FrameOptions configures single-frame capture to a writer or file.
type FrameOptions struct {
	Input       InputSource
	RTSP        *RTSPInputOptions
	At          FramePosition
	DurationSec float64
	Accuracy    CaptureAccuracy
	Width       int
	Height      int
	ScaleMode   ScaleMode
	Format      FrameFormat
	Quality     int
	LightProbe  *CaptureLightProbe
	Timeout     time.Duration
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

var (
	errFrameFormatUnsupported = errors.New("frame capture: image format encoder unavailable")
)

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
	// Stay slightly before EOF so ffmpeg can still decode one video frame.
	const epsilon = 0.05
	if durationSec > epsilon {
		return durationSec - epsilon
	}
	return 0
}

type frameEncoder struct {
	vcodec string
	format string
}

func resolveFrameEncoder(caps *capabilities.Capabilities, format FrameFormat) (frameEncoder, error) {
	switch format {
	case FrameWebP:
		if caps != nil {
			if caps.EncoderAvailable("libwebp") {
				return frameEncoder{vcodec: "libwebp", format: "webp"}, nil
			}
			if caps.EncoderAvailable("webp") {
				return frameEncoder{vcodec: "webp", format: "webp"}, nil
			}
			return frameEncoder{}, errFrameFormatUnsupported
		}
		return frameEncoder{vcodec: "libwebp", format: "webp"}, nil
	case FramePNG:
		if caps != nil && !caps.EncoderAvailable("png") {
			return frameEncoder{}, errFrameFormatUnsupported
		}
		return frameEncoder{vcodec: "png", format: "image2"}, nil
	default:
		if caps != nil && !caps.EncoderAvailable("mjpeg") {
			return frameEncoder{}, errFrameFormatUnsupported
		}
		return frameEncoder{vcodec: "mjpeg", format: "image2"}, nil
	}
}

func frameQualityQScale(format FrameFormat, quality int) string {
	q := quality
	switch format {
	case FrameWebP, FramePNG:
		if q == 0 {
			q = 80
		}
		if q < 1 {
			q = 1
		}
		if q > 100 {
			q = 100
		}
		// Map 1–100 to ffmpeg 1–31-ish scale for -q:v on webp/png.
		return encode.QualityToQScale(1 + (q-1)*9/99)
	default:
		return encode.QualityToQScale(q)
	}
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

	var dest io.Writer = w
	var buf *bytes.Buffer
	if opts.LightProbe != nil {
		buf = &bytes.Buffer{}
		dest = buf
	}

	args, err := buildCaptureFrameArgs(runner, opts, enc, seekSec)
	if err != nil {
		return err
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := ffexec.CommandContext(pctx, runner.FFmpegPath, args...)
	cmd.Stdout = dest
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if pctx.Err() != nil {
			return pctx.Err()
		}
		return fmt.Errorf("%w: %s", err, stderr.String())
	}

	if buf != nil {
		if opts.LightProbe != nil {
			if luma, err := meanLumaFromImage(buf.Bytes()); err == nil {
				opts.LightProbe.MeanLuma = luma
			}
		}
		if _, err := io.Copy(w, buf); err != nil {
			return err
		}
	}
	return nil
}

func buildCaptureFrameArgs(runner *ffexec.Runner, opts FrameOptions, enc frameEncoder, seekSec float64) ([]string, error) {
	args := []string{"-hide_banner", "-nostats"}
	accurate := opts.Accuracy == CaptureExact
	if !accurate && seekSec > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", seekSec))
	}
	args = appendInputFlags(args, runner, opts.Input, opts.RTSP, nil)
	args = append(args, "-i", opts.Input.URL)
	if accurate && seekSec > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", seekSec))
	}
	if opts.Width > 0 && opts.Height > 0 {
		args = append(args, "-vf", PreviewScaleFilter(opts.Width, opts.Height, opts.ScaleMode))
	}
	q := frameQualityQScale(opts.Format, opts.Quality)
	args = append(args,
		"-frames:v", "1",
		"-q:v", q,
		"-vcodec", enc.vcodec,
		"-f", enc.format,
		"-loglevel", "error",
		"-",
	)
	return args, nil
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

	target := path
	var cleanup func()
	if atomic {
		tmp, err := os.CreateTemp(dir, ".goffmpeg-capture-*")
		if err != nil {
			return err
		}
		target = tmp.Name()
		cleanup = func() {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
		defer cleanup()
		if err := CaptureFrame(ctx, runner, caps, tmp, opts, seekSec); err != nil {
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		if err := os.Rename(target, path); err != nil {
			return err
		}
		cleanup = func() {} // success: do not remove published file
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
	return nil
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
			// ITU-R BT.601 luma in 0–255.
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

// ScreenshotOptionsToFrame maps legacy screenshot options (first frame, JPEG).
func ScreenshotOptionsToFrame(opts ScreenshotOptions) FrameOptions {
	return FrameOptions{
		Input:   opts.Input,
		RTSP:    opts.RTSP,
		Quality: opts.Quality,
		Timeout: opts.Timeout,
		Format:  FrameJPEG,
	}
}

// PreviewOptionsToFrame maps legacy preview options to FrameOptions (MJPEG stream).
func PreviewOptionsToFrame(opts PreviewOptions, seekSec float64) FrameOptions {
	q := opts.Quality
	if q == 0 {
		q = 10
	}
	return FrameOptions{
		Input:     InputSource{URL: opts.Input, StreamType: probe.StreamFile},
		At:        AtSec(seekSec),
		Width:     opts.Width,
		Height:    opts.Height,
		ScaleMode: opts.ScaleMode,
		Quality:   q,
		Timeout:   opts.Timeout,
		Format:    FrameJPEG,
	}
}

// ClampPreviewPercent documents legacy preview percent handling (default 10% when unset).
func ClampPreviewPercent(pct float64) float64 {
	if pct <= 0 || pct > 100 {
		if pct > 100 {
			return 100
		}
		return 10
	}
	return pct
}

func previewSeekFromLegacy(dur float64, opts PreviewOptions) float64 {
	if opts.SeekSec > 0 {
		seekSec := opts.SeekSec
		if dur > 0 && seekSec > dur {
			seekSec = dur
		}
		return seekSec
	}
	pct := ClampPreviewPercent(opts.SeekPercent)
	if dur <= 0 {
		return 0
	}
	return dur * pct / 100
}

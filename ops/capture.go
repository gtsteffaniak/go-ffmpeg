package ops

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

// CaptureAccuracy controls how closely frame capture matches the requested timestamp.
type CaptureAccuracy int

const (
	// CaptureFast seeks before demux and decodes from the nearest prior keyframe (default for thumbnails).
	CaptureFast CaptureAccuracy = 0
	// CaptureExact input-seeks then decodes forward to the requested time (-accurate_seek).
	CaptureExact CaptureAccuracy = 1
)

const (
	// CaptureDefaultLightProbeSize is ffmpeg -probesize for lightweight capture when
	// duration/seek is already known. Well below libavformat defaults (~5M) but large
	// enough for MPEG-TS, late-stream MKV, and brittle AVI indexes (32KiB breaks these).
	CaptureDefaultLightProbeSize = "3145728" // 3 MiB
	// CaptureDefaultLightAnalyzeDuration is ffmpeg -analyzeduration in microseconds.
	CaptureDefaultLightAnalyzeDuration = "1500000" // 1.5s

	captureSWSFlags         = "fast_bilinear"
	captureDefaultThreads   = "1"
	captureImagePipeBufSize = 256 * 1024
)

// CaptureLightProbe optionally overrides lightweight input probing for frame capture.
// Empty fields use CaptureDefaultLightProbeSize and CaptureDefaultLightAnalyzeDuration.
type CaptureLightProbe struct {
	ProbeSize       string
	AnalyzeDuration string
}

var capturePipeBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, captureImagePipeBufSize)
		return &b
	},
}

// ResolveCaptureSeekSec picks a seek position from duration and explicit options.
func ResolveCaptureSeekSec(dur float64, seekSec float64, seekPercent float64) float64 {
	if seekSec > 0 {
		if dur > 0 && seekSec > dur {
			return dur
		}
		return seekSec
	}
	if seekPercent > 0 {
		if seekPercent > 100 {
			seekPercent = 100
		}
		if dur <= 0 {
			return 0
		}
		return dur * seekPercent / 100
	}
	return 0
}

func appendCaptureSeekArgs(args []string, seekSec float64, accuracy CaptureAccuracy) []string {
	if seekSec > 0 {
		args = append(args, "-ss", formatCaptureSeek(seekSec))
	}
	switch accuracy {
	case CaptureExact:
		args = append(args, "-accurate_seek")
	default:
		args = append(args, "-noaccurate_seek")
	}
	return args
}

func lightProbeValues(custom *CaptureLightProbe) (probeSize, analyzeDuration string) {
	probeSize = CaptureDefaultLightProbeSize
	analyzeDuration = CaptureDefaultLightAnalyzeDuration
	if custom != nil {
		if custom.ProbeSize != "" {
			probeSize = custom.ProbeSize
		}
		if custom.AnalyzeDuration != "" {
			analyzeDuration = custom.AnalyzeDuration
		}
	}
	return probeSize, analyzeDuration
}

func appendCaptureInputTuning(args []string, skipHeavyProbe bool, keyframeOnly bool, lightProbe *CaptureLightProbe) []string {
	if skipHeavyProbe {
		ps, ad := lightProbeValues(lightProbe)
		args = append(args, "-probesize", ps, "-analyzeduration", ad)
	}
	if keyframeOnly {
		args = append(args, "-skip_frame", "nokey")
	}
	return args
}

func appendCaptureOutputTuning(args []string) []string {
	return append(args,
		"-sws_flags", captureSWSFlags,
		"-an", "-sn", "-dn",
		"-threads", captureDefaultThreads,
	)
}

func formatCaptureSeek(sec float64) string {
	return fmt.Sprintf("%.3f", sec)
}

// FrameCaptureArgs bundles argv for a single-frame capture (tests / benchmarks).
type FrameCaptureArgs struct {
	SeekSec      float64
	Accuracy     CaptureAccuracy
	SkipProbe    bool
	LightProbe   *CaptureLightProbe
	KeyframeOnly bool
	ScaleFilter  string
	QScale       string
	// MJPEG selects mjpeg + image2pipe when VideoCodec/MuxerFormat are unset.
	MJPEG        bool
	VideoCodec   string
	MuxerFormat  string
}

// BuildFrameCaptureArgs assembles ffmpeg argv for one image frame to stdout (image2pipe).
func BuildFrameCaptureArgs(runner *ffexec.Runner, input InputSource, rtsp *RTSPInputOptions, cfg FrameCaptureArgs) []string {
	args := []string{"-hide_banner", "-nostats"}
	args = appendCaptureSeekArgs(args, cfg.SeekSec, cfg.Accuracy)
	args = appendCaptureInputTuning(args, cfg.SkipProbe, cfg.KeyframeOnly, cfg.LightProbe)
	args = appendInputFlags(args, runner, input, rtsp, nil)
	args = append(args, "-i", input.URL)
	args = appendCaptureOutputTuning(args)
	if cfg.ScaleFilter != "" {
		args = append(args, "-vf", cfg.ScaleFilter)
	}
	args = append(args, "-frames:v", "1")
	if cfg.QScale != "" {
		args = append(args, "-q:v", cfg.QScale)
	}
	vcodec := cfg.VideoCodec
	muxer := cfg.MuxerFormat
	if cfg.MJPEG {
		if vcodec == "" {
			vcodec = "mjpeg"
		}
		if muxer == "" {
			muxer = "image2pipe"
		}
	}
	if muxer == "" {
		muxer = "image2pipe"
	}
	if vcodec != "" {
		args = append(args, "-vcodec", vcodec)
	}
	args = append(args, "-f", muxer, "-")
	return args
}

func runFrameCaptureToWriter(ctx context.Context, runner *ffexec.Runner, args []string, w io.Writer) error {
	cmd := ffexec.CommandContext(ctx, runner.FFmpegPath, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	pr, pw := io.Pipe()
	cmd.Stdout = pw

	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		_ = pr.Close()
		return err
	}

	copyErr := make(chan error, 1)
	go func() {
		bufPtr := capturePipeBufPool.Get().(*[]byte)
		defer capturePipeBufPool.Put(bufPtr)
		_, err := io.CopyBuffer(w, pr, *bufPtr)
		copyErr <- err
	}()

	waitErr := cmd.Wait()
	_ = pw.Close()
	copyDone := <-copyErr
	_ = pr.Close()

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if waitErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", waitErr, msg)
		}
		return waitErr
	}
	return copyDone
}

func runFrameCaptureToFile(ctx context.Context, runner *ffexec.Runner, args []string, path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := runFrameCaptureToWriter(ctx, runner, args, f); err != nil {
		return err
	}
	return f.Sync()
}

func screenshotScaleFilter(opts ScreenshotOptions) string {
	if opts.Width <= 0 || opts.Height <= 0 {
		return ""
	}
	return PreviewScaleFilter(opts.Width, opts.Height, opts.ScaleMode)
}

func buildScreenshotArgs(runner *ffexec.Runner, opts ScreenshotOptions, seekSec float64) []string {
	q := opts.Quality
	if q == 0 {
		q = 85
	}
	accuracy := opts.Accuracy
	skipProbe := opts.DurationSec > 0 || opts.SeekSec > 0 || opts.SeekPercent > 0
	return BuildFrameCaptureArgs(runner, opts.Input, opts.RTSP, FrameCaptureArgs{
		SeekSec:      seekSec,
		Accuracy:     accuracy,
		SkipProbe:    skipProbe,
		LightProbe:   opts.LightProbe,
		KeyframeOnly: accuracy == CaptureFast,
		ScaleFilter:  screenshotScaleFilter(opts),
		QScale:       encode.QualityToQScale(q),
		MJPEG:        true,
	})
}

package ops

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// SpriteFormat is the still image encoding for a sprite sheet.
type SpriteFormat string

const (
	SpriteFormatJPEG SpriteFormat = "jpeg"
	SpriteFormatWebP SpriteFormat = "webp"
)

// SpriteSheetOptions configures thumbnail sprite generation in one ffmpeg process.
type SpriteSheetOptions struct {
	Input      InputSource
	OutputPath string
	WebVTTPath string // optional sidecar for player scrub previews

	TileWidth  int
	TileHeight int
	Columns    int
	Rows       int
	// TileCount when > 0 overrides Columns*Rows (grid is derived from Columns when Rows is zero).
	TileCount int

	IntervalSec float64 // when > 0, sample every IntervalSec (fps=1/interval)
	StartSec    float64
	EndSec      float64 // 0 = full duration

	DurationSec float64
	Decode      encode.VideoDecodeProfile
	// KeyframeOnly limits decode to keyframes when true (default when IntervalSec >= 2).
	KeyframeOnly *bool
	Accuracy     CaptureAccuracy

	Format  SpriteFormat
	Quality int
	RTSP    *RTSPInputOptions
	Timeout time.Duration
}

// SpriteSheetResult describes the generated sprite sheet.
type SpriteSheetResult struct {
	OutputPath   string
	WebVTTPath   string
	Columns      int
	Rows         int
	TileWidth    int
	TileHeight   int
	TileCount    int
	IntervalSec  float64
	DecodeAccel  capabilities.AccelType
	DecodeDriver string
}

// SpriteSheetCapabilities exposes whether hardware decode can run sprite jobs.
type SpriteSheetCapabilities struct {
	HardwareDecodeAvailable bool
	PreferredDecode         capabilities.DecodeOption
	AvailableDecodes        []capabilities.DecodeOption
}

// SpriteSheetCapabilitiesFrom reports decode paths usable for sprite generation.
func SpriteSheetCapabilitiesFrom(caps *capabilities.Capabilities) SpriteSheetCapabilities {
	if caps == nil {
		return SpriteSheetCapabilities{}
	}
	available := caps.AvailableDecodeOptions()
	var preferred capabilities.DecodeOption
	for _, opt := range available {
		if opt.Default {
			preferred = opt
			break
		}
	}
	if preferred.Decoder == "" && len(available) > 0 {
		preferred = available[0]
	}
	hw := preferred.Accel != "" && preferred.Accel != capabilities.AccelNone
	return SpriteSheetCapabilities{
		HardwareDecodeAvailable: hw,
		PreferredDecode:         preferred,
		AvailableDecodes:        available,
	}
}

// SpriteSheet generates a tiled sprite image (and optional WebVTT) in one ffmpeg invocation.
func SpriteSheet(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts SpriteSheetOptions) (SpriteSheetResult, error) {
	resolver := encode.NewResolver(caps)
	decodeArgs, err := resolver.VideoDecoderArgs(opts.Decode)
	if err != nil {
		return SpriteSheetResult{}, err
	}
	sel, _ := resolver.ResolveDecoder(opts.Decode)

	layout, interval, err := resolveSpriteLayout(ctx, runner, opts)
	if err != nil {
		return SpriteSheetResult{}, err
	}

	keyframeOnly := spriteKeyframeOnly(opts, interval)
	filter := spriteSheetFilter(resolver, opts.Decode, sel, layout, interval, keyframeOnly)

	args := []string{"-hide_banner", "-nostats"}
	args = appendCaptureSeekArgs(args, layout.StartSec, opts.Accuracy)
	args = appendCaptureInputTuning(args, opts.DurationSec > 0, keyframeOnly)
	args = appendInputFlags(args, runner, opts.Input, opts.RTSP, nil)
	args = append(args, decodeArgs...)
	args = append(args, "-i", opts.Input.URL)
	if layout.TileCount > 0 && interval > 0 {
		span := interval * float64(layout.TileCount-1)
		if span < 0.1 {
			span = 0.1
		}
		args = append(args, "-t", formatCaptureSeek(span+interval*0.5))
	}
	args = appendCaptureOutputTuning(args)
	args = append(args, "-vf", filter)
	args = append(args, "-frames:v", "1")

	format := opts.Format
	if format == "" {
		format = SpriteFormatJPEG
	}
	q := opts.Quality
	if q == 0 {
		q = 85
	}
	switch format {
	case SpriteFormatWebP:
		args = append(args, "-c:v", "libwebp", "-quality", fmt.Sprintf("%d", q))
	default:
		args = append(args, "-q:v", encode.QualityToQScale(q))
	}
	args = append(args, "-f", "image2", "-y", opts.OutputPath)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err = runner.RunFFmpeg(pctx, args...)
	if err != nil {
		return SpriteSheetResult{}, err
	}

	result := SpriteSheetResult{
		OutputPath:   opts.OutputPath,
		WebVTTPath:   opts.WebVTTPath,
		Columns:      layout.Columns,
		Rows:         layout.Rows,
		TileWidth:    layout.TileWidth,
		TileHeight:   layout.TileHeight,
		TileCount:    layout.TileCount,
		IntervalSec:  interval,
		DecodeAccel:  sel.Accel,
		DecodeDriver: sel.Decoder,
	}

	if opts.WebVTTPath != "" {
		if err := writeSpriteWebVTT(opts.WebVTTPath, filepath.Base(opts.OutputPath), layout, interval); err != nil {
			return result, err
		}
	}
	return result, nil
}

type spriteLayout struct {
	Columns   int
	Rows      int
	TileWidth int
	TileHeight int
	TileCount int
	StartSec  float64
	EndSec    float64
}

func resolveSpriteLayout(ctx context.Context, runner *ffexec.Runner, opts SpriteSheetOptions) (spriteLayout, float64, error) {
	tw := opts.TileWidth
	th := opts.TileHeight
	if tw <= 0 {
		tw = 160
	}
	if th <= 0 {
		th = 90
	}
	count := opts.TileCount
	if count <= 0 {
		count = opts.Columns * opts.Rows
	}
	cols := opts.Columns
	rows := opts.Rows
	if count <= 0 {
		count = 100
	}
	if cols <= 0 && rows <= 0 {
		cols = int(math.Ceil(math.Sqrt(float64(count))))
		if cols < 1 {
			cols = 1
		}
		rows = (count + cols - 1) / cols
	} else if cols <= 0 {
		cols = (count + rows - 1) / rows
	} else if rows <= 0 {
		rows = (count + cols - 1) / cols
	}
	if cols*rows < count {
		count = cols * rows
	}

	dur := opts.DurationSec
	if dur <= 0 {
		var err error
		dur, err = probe.GetMediaDuration(ctx, runner, opts.Input.URL)
		if err != nil {
			return spriteLayout{}, 0, err
		}
	}
	start := opts.StartSec
	end := opts.EndSec
	if end <= 0 || end > dur {
		end = dur
	}
	if end < start {
		end = start
	}

	interval := opts.IntervalSec
	if interval <= 0 {
		span := end - start
		if count > 1 && span > 0 {
			interval = span / float64(count-1)
		} else {
			interval = 1
		}
	}

	return spriteLayout{
		Columns:    cols,
		Rows:       rows,
		TileWidth:  tw,
		TileHeight: th,
		TileCount:  count,
		StartSec:   start,
		EndSec:     end,
	}, interval, nil
}

func spriteKeyframeOnly(opts SpriteSheetOptions, interval float64) bool {
	if opts.KeyframeOnly != nil {
		return *opts.KeyframeOnly
	}
	return interval >= 2.0
}

func spriteSheetFilter(r *encode.Resolver, decode encode.VideoDecodeProfile, sel capabilities.DecoderSelection, layout spriteLayout, interval float64, keyframeOnly bool) string {
	// prev_selected_t spaces samples by interval on the post-seek timeline (works across ffmpeg 6–8).
	selectExpr := fmt.Sprintf("isnan(prev_selected_t)+gte(t-prev_selected_t\\,%.6f)", interval)
	scale := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", layout.TileWidth, layout.TileHeight)
	tile := fmt.Sprintf("tile=%dx%d", layout.Columns, layout.Rows)

	hw := sel.Accel != capabilities.AccelNone && sel.Decoder != "" && !decode.ForceSoftware
	parts := []string{"select='" + selectExpr + "'"}
	if hw {
		parts = append(parts, spriteHWScaleChain(r, sel, layout.TileWidth, layout.TileHeight))
	} else {
		parts = append(parts, scale)
	}
	parts = append(parts, tile)
	if keyframeOnly {
		// select already limits output frames; skip_frame is applied at demux.
	}
	return strings.Join(parts, ",")
}

func spriteHWScaleChain(r *encode.Resolver, sel capabilities.DecoderSelection, w, h int) string {
	switch sel.Accel {
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		if r != nil && r.Caps != nil && r.Caps.FilterAvailable("scale_vaapi") {
			return fmt.Sprintf("scale_vaapi=w=%d:h=%d,hwdownload,format=nv12", w, h)
		}
		return fmt.Sprintf("hwdownload,format=nv12,scale=%d:%d:force_original_aspect_ratio=decrease", w, h)
	case capabilities.AccelVideoToolbox:
		if r != nil && r.Caps != nil && r.Caps.FilterAvailable("scale_vt") {
			return fmt.Sprintf("hwdownload,format=nv12,scale_vt=w=trunc(iw*min(%d\\,ih)/ih/2)*2:h=min(%d\\,ih)", h, h)
		}
		return fmt.Sprintf("hwdownload,format=nv12,scale=%d:%d:force_original_aspect_ratio=decrease", w, h)
	case capabilities.AccelNVENC:
		if r != nil && r.Caps != nil && r.Caps.FilterAvailable("scale_cuda") {
			return fmt.Sprintf("hwdownload,format=nv12,scale_cuda=%d:%d:force_original_aspect_ratio=decrease", w, h)
		}
		return fmt.Sprintf("hwdownload,format=nv12,scale=%d:%d:force_original_aspect_ratio=decrease", w, h)
	case capabilities.AccelQSV:
		return fmt.Sprintf("hwdownload,format=nv12,scale=%d:%d:force_original_aspect_ratio=decrease", w, h)
	default:
		return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", w, h)
	}
}

func writeSpriteWebVTT(path string, imageName string, layout spriteLayout, interval float64) error {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for i := 0; i < layout.TileCount; i++ {
		start := layout.StartSec + float64(i)*interval
		end := start + interval
		col := i % layout.Columns
		row := i / layout.Columns
		x := col * layout.TileWidth
		y := row * layout.TileHeight
		b.WriteString(formatVTTTime(start))
		b.WriteString(" --> ")
		b.WriteString(formatVTTTime(end))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%s#xywh=%d,%d,%d,%d\n\n", imageName, x, y, layout.TileWidth, layout.TileHeight))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func formatVTTTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	h := int(sec) / 3600
	m := (int(sec) % 3600) / 60
	s := int(sec) % 60
	ms := int((sec - float64(int(sec))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

// BuildSpriteSheetFilter exports the -vf filter chain for tests.
func BuildSpriteSheetFilter(caps *capabilities.Capabilities, decode encode.VideoDecodeProfile, layout spriteLayout, interval float64, keyframeOnly bool) (string, error) {
	resolver := encode.NewResolver(caps)
	sel, err := resolver.ResolveDecoder(decode)
	if err != nil {
		return "", err
	}
	return spriteSheetFilter(resolver, decode, sel, layout, interval, keyframeOnly), nil
}

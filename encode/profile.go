package encode

import (
	"strconv"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

// VideoCodec identifies output codec family.
type VideoCodec = capabilities.VideoCodec

const (
	CodecH264 = capabilities.CodecH264
	CodecAV1  = capabilities.CodecAV1
	CodecVP9  = capabilities.CodecVP9
	CodecHEVC = capabilities.CodecHEVC
	CodecCopy = VideoCodec("copy")
)

// QualityPreset maps to encoder preset strings.
type QualityPreset string

const (
	PresetUltrafast QualityPreset = "ultrafast"
	PresetVeryfast  QualityPreset = "veryfast"
	PresetFast      QualityPreset = "fast"
	PresetMedium    QualityPreset = "medium"
)

// BitrateConfig holds rate control settings.
type BitrateConfig struct {
	Target  string
	Min     string
	Max     string
	BufSize string
}

// VideoProfile configures video encoding.
type VideoProfile struct {
	Codec       VideoCodec
	Quality     QualityPreset
	Bitrate     BitrateConfig
	PixelFormat string
	GOP         int

	// Accel selects a hardware backend. Zero value uses the detected preferred path.
	// Set to AccelNone (or ForceSoftware) to require software encoding.
	Accel capabilities.AccelType

	// Encoder forces a specific ffmpeg encoder (e.g. libsvtav1, h264_qsv).
	// Must be available in the cached capability matrix.
	Encoder string

	// ForceSoftware is shorthand for software encoding without naming an encoder.
	ForceSoftware bool
}

type encodeOpts struct {
	quality QualityPreset
	pixFmt  string
	gop     int
}

// Resolver builds encoder arguments from capabilities.
type Resolver struct {
	Caps *capabilities.Capabilities
}

// NewResolver creates an encoder argument resolver.
func NewResolver(caps *capabilities.Capabilities) *Resolver {
	return &Resolver{Caps: caps}
}

// VideoEncoderArgs returns ffmpeg video encoder arguments for a profile.
func (r *Resolver) VideoEncoderArgs(profile VideoProfile) ([]string, error) {
	if profile.Codec == CodecCopy {
		return []string{"-c:v", "copy"}, nil
	}
	b := profile.Bitrate
	if b.Target == "" {
		b.Target = "2M"
	}
	if b.BufSize == "" {
		b.BufSize = b.Target
	}
	if b.Max == "" {
		b.Max = b.Target
	}

	sel, err := r.ResolveEncoder(profile)
	if err != nil {
		return nil, err
	}

	opts := profileEncodeOpts(profile)

	switch profile.Codec {
	case "", CodecH264:
		return h264Args(sel.Accel, sel.Encoder, b, opts), nil
	case CodecAV1:
		return av1Args(sel.Accel, sel.Encoder, b, opts), nil
	case CodecVP9:
		return vp9Args(sel.Accel, sel.Encoder, b, opts), nil
	case CodecHEVC:
		return hevcArgs(sel.Accel, sel.Encoder, b, opts), nil
	default:
		return []string{"-c:v", sel.Encoder, "-b:v", b.Target, "-bufsize", b.BufSize}, nil
	}
}

func profileEncodeOpts(p VideoProfile) encodeOpts {
	q := p.Quality
	if q == "" {
		q = defaultQuality(p.Codec)
	}
	pix := p.PixelFormat
	if pix == "" {
		pix = "yuv420p"
	}
	gop := p.GOP
	if gop <= 0 {
		gop = defaultGOP(p.Codec)
	}
	return encodeOpts{quality: q, pixFmt: pix, gop: gop}
}

func defaultQuality(codec VideoCodec) QualityPreset {
	if codec == CodecAV1 {
		return PresetMedium
	}
	return PresetVeryfast
}

func defaultGOP(codec VideoCodec) int {
	switch codec {
	case CodecAV1:
		return 240
	case CodecH264:
		return 60
	default:
		return 0
	}
}

func h264Args(accel capabilities.AccelType, encoder string, b BitrateConfig, o encodeOpts) []string {
	gop := strconv.Itoa(o.gop)
	switch accel {
	case capabilities.AccelNVENC:
		return []string{
			"-c:v", encoder, "-preset", nvencPreset(o.quality),
			"-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize,
			"-pix_fmt", o.pixFmt, "-g", gop,
		}
	case capabilities.AccelAMF:
		return []string{
			"-c:v", encoder, "-rc", "vbr_constrained",
			"-b:v", b.Target, "-maxrate", b.Max,
			"-quality_preset", amfQualityPreset(o.quality, CodecH264),
			"-pix_fmt", o.pixFmt, "-g", gop,
		}
	case capabilities.AccelQSV:
		return qsvEncoderArgs(encoder, o.quality, "-b:v", b.Target, "-maxrate", b.Max, "-g", gop)
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return []string{
			"-c:v", encoder, "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize,
			"-pix_fmt", o.pixFmt, "-g", gop,
		}
	case capabilities.AccelVideoToolbox:
		return []string{
			"-c:v", encoder, "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize,
			"-pix_fmt", o.pixFmt, "-g", gop,
		}
	default:
		args := []string{
			"-c:v", "libx264", "-preset", x264Preset(o.quality),
			"-x264-params", "nal-hrd=cbr:force-cfr=1",
			"-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize,
			"-pix_fmt", o.pixFmt, "-g", gop, "-tune", "zerolatency",
		}
		if b.Min != "" {
			args = append(args, "-minrate", b.Min)
		}
		if encoder != "" && encoder != "libx264" {
			args[1] = encoder
		}
		return args
	}
}

func av1Args(accel capabilities.AccelType, encoder string, b BitrateConfig, o encodeOpts) []string {
	gop := strconv.Itoa(o.gop)
	switch accel {
	case capabilities.AccelNVENC:
		return []string{
			"-c:v", encoder, "-rc", "vbr", "-b:v", b.Target, "-bufsize", b.BufSize,
			"-preset", nvencPreset(o.quality), "-g", gop, "-pix_fmt", o.pixFmt,
		}
	case capabilities.AccelAMF:
		return []string{
			"-c:v", encoder, "-rc", "vbr_constrained", "-b:v", b.Target,
			"-quality_preset", amfQualityPreset(o.quality, CodecAV1),
			"-g", gop, "-pix_fmt", o.pixFmt,
		}
	case capabilities.AccelQSV:
		return qsvEncoderArgs(encoder, o.quality, "-b:v", b.Target, "-maxrate", b.Max, "-g", gop)
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return []string{
			"-c:v", encoder, "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize,
			"-g", gop, "-pix_fmt", o.pixFmt,
		}
	default:
		return []string{
			"-c:v", "libsvtav1", "-b:v", b.Target,
			"-preset", svtav1Preset(o.quality), "-g", gop, "-pix_fmt", o.pixFmt,
			"-svtav1-params", "tune=0:fast-decode=1:film-grain=0",
		}
	}
}

func vp9Args(accel capabilities.AccelType, encoder string, b BitrateConfig, o encodeOpts) []string {
	switch accel {
	case capabilities.AccelNVENC:
		return []string{
			"-c:v", encoder, "-rc", "vbr", "-b:v", b.Target, "-maxrate", b.Max,
			"-bufsize", b.BufSize, "-preset", nvencPreset(o.quality),
		}
	case capabilities.AccelAMF:
		return []string{
			"-c:v", encoder, "-rc", "vbr_constrained", "-b:v", b.Target, "-maxrate", b.Max,
			"-quality_preset", amfQualityPreset(o.quality, CodecVP9),
		}
	case capabilities.AccelQSV:
		return qsvEncoderArgs(encoder, o.quality, "-b:v", b.Target, "-maxrate", b.Max)
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return []string{"-c:v", encoder, "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize}
	default:
		return []string{"-c:v", "libvpx-vp9", "-b:v", b.Target, "-bufsize", b.BufSize, "-deadline", vp9Deadline(o.quality)}
	}
}

func hevcArgs(accel capabilities.AccelType, encoder string, b BitrateConfig, o encodeOpts) []string {
	switch accel {
	case capabilities.AccelNVENC:
		return []string{
			"-c:v", encoder, "-preset", nvencPreset(o.quality),
			"-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize, "-pix_fmt", o.pixFmt,
		}
	case capabilities.AccelQSV:
		return qsvEncoderArgs(encoder, o.quality, "-b:v", b.Target, "-maxrate", b.Max)
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return []string{"-c:v", encoder, "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize, "-pix_fmt", o.pixFmt}
	case capabilities.AccelVideoToolbox:
		return []string{"-c:v", encoder, "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize, "-pix_fmt", o.pixFmt}
	default:
		preset := x265Preset(o.quality)
		if encoder == "libx265" {
			return []string{"-c:v", encoder, "-b:v", b.Target, "-preset", preset, "-pix_fmt", o.pixFmt}
		}
		return []string{"-c:v", encoder, "-b:v", b.Target, "-preset", preset, "-pix_fmt", o.pixFmt}
	}
}

// qsvEncoderArgs builds Intel QSV encoder flags for oneVPL/FFmpeg 8+ (numeric preset, nv12).
// Low-latency settings for on-demand HLS: no lookahead, shallow async queue, no B-frames.
func qsvEncoderArgs(encoder string, preset QualityPreset, tail ...string) []string {
	args := []string{
		"-c:v", encoder,
		"-preset", qsvPreset(preset),
		"-pix_fmt", "nv12",
		"-look_ahead_depth", "0",
		"-async_depth", "1",
		"-bf", "0",
		"-low_power", "1",
	}
	return append(args, tail...)
}

func x264Preset(p QualityPreset) string {
	switch p {
	case PresetUltrafast, PresetVeryfast, PresetFast, PresetMedium:
		return string(p)
	default:
		return string(PresetVeryfast)
	}
}

func x265Preset(p QualityPreset) string {
	switch p {
	case PresetUltrafast, PresetVeryfast:
		return "ultrafast"
	case PresetFast:
		return "fast"
	case PresetMedium:
		return "medium"
	default:
		return "ultrafast"
	}
}

func svtav1Preset(p QualityPreset) string {
	switch p {
	case PresetUltrafast:
		return "12"
	case PresetVeryfast:
		return "10"
	case PresetFast:
		return "8"
	case PresetMedium:
		return "6"
	default:
		return "8"
	}
}

func vp9Deadline(p QualityPreset) string {
	if p == PresetUltrafast || p == PresetVeryfast {
		return "realtime"
	}
	return "good"
}

// nvencPreset maps quality presets to NVENC -preset values.
func nvencPreset(p QualityPreset) string {
	switch p {
	case PresetUltrafast:
		return "ultrafast"
	case PresetVeryfast, PresetFast:
		return "fast"
	case PresetMedium:
		return "medium"
	default:
		return "fast"
	}
}

// amfQualityPreset maps quality presets to AMF -quality_preset values.
func amfQualityPreset(p QualityPreset, codec VideoCodec) string {
	if codec == CodecAV1 {
		switch p {
		case PresetUltrafast, PresetVeryfast, PresetFast:
			return "speed"
		default:
			return "quality"
		}
	}
	switch p {
	case PresetUltrafast, PresetVeryfast:
		return "speed"
	case PresetFast:
		return "balanced"
	default:
		return "quality"
	}
}

// qsvPreset maps x264-style preset names to Intel QSV numeric presets (0=slowest, 7=fastest).
func qsvPreset(preset QualityPreset) string {
	switch preset {
	case PresetUltrafast, PresetVeryfast:
		return "7"
	case PresetFast:
		return "6"
	case PresetMedium:
		return "4"
	default:
		return "7"
	}
}

// QualityToQScale maps a 1-10 quality level to ffmpeg -q:v (lower is better).
func QualityToQScale(quality int) string {
	if quality < 1 {
		quality = 1
	}
	if quality > 10 {
		quality = 10
	}
	return strconv.Itoa(quality)
}

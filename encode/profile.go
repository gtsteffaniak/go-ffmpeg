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

	switch profile.Codec {
	case "", CodecH264:
		return h264Args(sel.Accel, sel.Encoder, b), nil
	case CodecAV1:
		return av1Args(sel.Accel, sel.Encoder, b), nil
	case CodecVP9:
		return vp9Args(sel.Accel, sel.Encoder, b), nil
	case CodecHEVC:
		return hevcArgs(sel.Accel, sel.Encoder, b), nil
	default:
		return []string{"-c:v", sel.Encoder, "-b:v", b.Target, "-bufsize", b.BufSize}, nil
	}
}

func h264Args(accel capabilities.AccelType, encoder string, b BitrateConfig) []string {
	switch accel {
	case capabilities.AccelNVENC:
		return []string{"-c:v", encoder, "-preset", "fast", "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize, "-pix_fmt", "yuv420p"}
	case capabilities.AccelAMF:
		return []string{"-c:v", encoder, "-rc", "vbr_constrained", "-b:v", b.Target, "-maxrate", b.Max, "-quality_preset", "ultrafast", "-pix_fmt", "yuv420p"}
	case capabilities.AccelQSV:
		return []string{"-c:v", encoder, "-load_plugin", "1", "-preset", "ultrafast", "-b:v", b.Target, "-maxrate", b.Max, "-pix_fmt", "yuv420p"}
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return []string{"-c:v", encoder, "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize, "-pix_fmt", "yuv420p"}
	default:
		args := []string{
			"-c:v", "libx264", "-preset", "veryfast",
			"-x264-params", "nal-hrd=cbr:force-cfr=1",
			"-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize,
			"-pix_fmt", "yuv420p", "-g", "60", "-tune", "zerolatency",
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

func av1Args(accel capabilities.AccelType, encoder string, b BitrateConfig) []string {
	gop := "240"
	switch accel {
	case capabilities.AccelNVENC:
		return []string{"-c:v", encoder, "-rc", "vbr", "-b:v", b.Target, "-bufsize", b.BufSize, "-preset", "medium", "-g", gop, "-pix_fmt", "yuv420p"}
	case capabilities.AccelAMF:
		return []string{"-c:v", encoder, "-rc", "vbr_constrained", "-b:v", b.Target, "-quality_preset", "quality", "-g", gop, "-pix_fmt", "yuv420p"}
	case capabilities.AccelQSV:
		return []string{"-c:v", encoder, "-load_plugin", "1", "-b:v", b.Target, "-maxrate", b.Max, "-preset", "medium", "-g", gop, "-pix_fmt", "yuv420p"}
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return []string{"-c:v", encoder, "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize, "-g", gop, "-pix_fmt", "yuv420p"}
	default:
		return []string{"-c:v", "libsvtav1", "-b:v", b.Target, "-preset", "8", "-g", gop, "-pix_fmt", "yuv420p", "-svtav1-params", "tune=0:fast-decode=1:film-grain=0"}
	}
}

func vp9Args(accel capabilities.AccelType, encoder string, b BitrateConfig) []string {
	switch accel {
	case capabilities.AccelNVENC:
		return []string{"-c:v", encoder, "-rc", "vbr", "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize, "-preset", "ultrafast"}
	case capabilities.AccelAMF:
		return []string{"-c:v", encoder, "-rc", "vbr_constrained", "-b:v", b.Target, "-maxrate", b.Max, "-quality_preset", "ultrafast"}
	case capabilities.AccelQSV:
		return []string{"-c:v", encoder, "-load_plugin", "1", "-b:v", b.Target, "-maxrate", b.Max, "-preset", "ultrafast"}
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return []string{"-c:v", encoder, "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize}
	default:
		return []string{"-c:v", "libvpx-vp9", "-b:v", b.Target, "-bufsize", b.BufSize, "-deadline", "realtime"}
	}
}

func hevcArgs(accel capabilities.AccelType, encoder string, b BitrateConfig) []string {
	switch accel {
	case capabilities.AccelNVENC:
		return []string{"-c:v", encoder, "-preset", "fast", "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize, "-pix_fmt", "yuv420p"}
	case capabilities.AccelQSV:
		return []string{"-c:v", encoder, "-load_plugin", "1", "-b:v", b.Target, "-maxrate", b.Max, "-pix_fmt", "yuv420p"}
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return []string{"-c:v", encoder, "-b:v", b.Target, "-maxrate", b.Max, "-bufsize", b.BufSize, "-pix_fmt", "yuv420p"}
	default:
		preset := "8"
		if encoder == "libx265" {
			preset = "ultrafast"
		}
		return []string{"-c:v", encoder, "-b:v", b.Target, "-preset", preset, "-pix_fmt", "yuv420p"}
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

package ops

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

const defaultHLSSegmentFPS = 30

// minHLSSegmentMediaBytes rejects empty fMP4 fragments (moof+mdat shell only).
const minHLSSegmentMediaBytes = 8192

// minHLSTSBytes rejects empty MPEG-TS segments.
const minHLSTSBytes = 4096

// HLSSegmentOptions configures on-demand fMP4 HLS segment generation.
type HLSSegmentOptions struct {
	Input       InputSource
	StartSec    float64
	DurationSec float64
	Decode      encode.VideoDecodeProfile
	Profile     encode.VideoProfile
	MaxHeight   int
	Remux       bool
	// VideoCopy stream-copies H.264 video and transcodes audio to AAC (e.g. EAC3 in MKV).
	VideoCopy   bool
	AccurateSeek bool
	GOP         int
	VideoOnly   bool
}

// HLSSegment generates a self-contained MPEG-TS segment for full re-encode on-demand HLS.
// Each segment is an independent ffmpeg run; players stitch them with #EXT-X-DISCONTINUITY.
func HLSSegment(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSSegmentOptions) ([]byte, error) {
	if hlsUsesVideoCopyPipeline(opts) {
		return nil, fmt.Errorf("HLSSegment is for full transcode only")
	}
	raw, err := runHLSSegmentWithSeekRetry(ctx, runner, caps, opts, false)
	if err != nil {
		return nil, err
	}
	if err := validateHLSTS(raw, opts); err != nil {
		return nil, err
	}
	return raw, nil
}

// HLSInitAndSegment generates init bytes and media bytes in one encode pass for segment index 0.
func HLSInitAndSegment(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSSegmentOptions) (init, media []byte, err error) {
	if !hlsUsesVideoCopyPipeline(opts) {
		return nil, nil, fmt.Errorf("HLSInitAndSegment is for fMP4 copy/remux only; use HLSSegment for full transcode")
	}
	raw, err := runHLSSegmentWithSeekRetry(ctx, runner, caps, opts, true)
	if err != nil {
		return nil, nil, err
	}
	init, media, err = mp4.SplitInitMedia(raw)
	if err != nil {
		return nil, nil, err
	}
	if err = validateHLSSegmentMedia(media, opts); err != nil {
		return nil, nil, err
	}
	return init, media, nil
}

func validateHLSSegmentMedia(media []byte, opts HLSSegmentOptions) error {
	if len(media) < minHLSSegmentMediaBytes {
		return fmt.Errorf("segment media too small (%d bytes) at start %.3fs", len(media), opts.StartSec)
	}
	return nil
}

func validateHLSTS(data []byte, opts HLSSegmentOptions) error {
	if len(data) < minHLSTSBytes {
		return fmt.Errorf("ts segment too small (%d bytes) at start %.3fs", len(data), opts.StartSec)
	}
	if data[0] != 0x47 {
		return fmt.Errorf("ts segment missing sync byte at start %.3fs", opts.StartSec)
	}
	return nil
}

// HLSSegmentMedia generates a media-only fMP4 fragment (moof+mdat) for later segment requests.
func HLSSegmentMedia(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, w io.Writer, opts HLSSegmentOptions) error {
	if !hlsUsesVideoCopyPipeline(opts) {
		return fmt.Errorf("HLSSegmentMedia is for fMP4 copy/remux only; use HLSSegment for full transcode")
	}
	raw, err := runHLSSegmentWithSeekRetry(ctx, runner, caps, opts, false)
	if err != nil {
		return err
	}
	_, media, splitErr := mp4.SplitInitMedia(raw)
	if splitErr != nil {
		return splitErr
	}
	if len(media) == 0 {
		media = raw
	}
	if err := validateHLSSegmentMedia(media, opts); err != nil {
		return err
	}
	_, err = w.Write(media)
	return err
}

func runHLSSegmentWithSeekRetry(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSSegmentOptions, includeInit bool) ([]byte, error) {
	if opts.Remux || opts.VideoCopy || opts.StartSec <= 0 {
		return runHLSSegmentRaw(ctx, runner, caps, opts, includeInit)
	}

	// Full re-encode: always use input-accurate seek for mid-file segments. Fast input
	// seek can yield moof fragments that pass size checks but contain no parseable samples.
	accurate := opts
	accurate.AccurateSeek = true
	raw, err := runHLSSegmentRaw(ctx, runner, caps, accurate, includeInit)
	if err != nil {
		return nil, err
	}
	if !hlsUsesVideoCopyPipeline(opts) {
		if err := validateHLSTS(raw, opts); err != nil {
			return nil, err
		}
		return raw, nil
	}
	_, media, splitErr := mp4.SplitInitMedia(raw)
	if splitErr != nil {
		return nil, splitErr
	}
	if len(media) == 0 {
		media = raw
	}
	if err := validateHLSSegmentMedia(media, opts); err != nil {
		return nil, err
	}
	return raw, nil
}

func hlsUsesVideoCopyPipeline(opts HLSSegmentOptions) bool {
	return opts.Remux || opts.VideoCopy
}

func runHLSSegmentRaw(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSSegmentOptions, includeInit bool) ([]byte, error) {
	dur := opts.DurationSec
	if dur <= 0 {
		dur = 4
	}
	gop := opts.GOP
	if gop <= 0 {
		gop = int(defaultHLSSegmentFPS * dur)
	}

	args := []string{"-hide_banner", "-nostats", "-y"}
	videoCopyPipeline := hlsUsesVideoCopyPipeline(opts)
	if opts.StartSec > 0 {
		if videoCopyPipeline || !opts.AccurateSeek {
			args = append(args, "-ss", fmt.Sprintf("%.3f", opts.StartSec))
		}
	}
	args = appendInputFlags(args, opts.Input, nil)
	if !videoCopyPipeline {
		resolver := encode.NewResolver(caps)
		decodeArgs, err := resolver.VideoDecoderArgs(opts.Decode)
		if err != nil {
			return nil, err
		}
		args = append(args, decodeArgs...)
	}
	args = append(args, "-i", opts.Input.URL)
	if !videoCopyPipeline && opts.StartSec > 0 && opts.AccurateSeek {
		args = append(args, "-ss", fmt.Sprintf("%.3f", opts.StartSec))
	}
	args = append(args, "-map", "0:v:0")
	if !opts.VideoOnly {
		args = append(args, "-map", "0:a:0?")
	}
	args = append(args, "-sn", "-dn")

	switch {
	case opts.Remux:
		args = append(args, "-c:v", "copy")
		if !opts.VideoOnly {
			args = append(args, "-c:a", "copy")
		}
	case opts.VideoCopy:
		args = append(args, "-c:v", "copy")
		if !opts.VideoOnly {
			args = append(args,
				"-c:a", "aac",
				"-ar", "48000",
				"-ac", "2",
				"-profile:a", "aac_low",
				"-af", "aresample=async=1:first_pts=0",
				// Required for AAC in fMP4/MSE; without it browsers play video-only.
				"-bsf:a", "aac_adtstoasc",
			)
		}
	default:
		resolver := encode.NewResolver(caps)
		vidArgs, err := resolver.VideoEncoderArgs(opts.Profile)
		if err != nil {
			return nil, err
		}
		if opts.MaxHeight > 0 {
			args = append(args, "-vf", fmt.Sprintf("scale=-2:min(%d\\,ih)", opts.MaxHeight))
		}
		args = append(args, vidArgs...)
		if opts.Profile.Codec == "" || opts.Profile.Codec == encode.CodecH264 {
			args = append(args, "-profile:v", "baseline", "-level", "3.1", "-tag:v", "avc1")
		}
		args = append(args,
			"-g", fmt.Sprintf("%d", gop),
			"-keyint_min", fmt.Sprintf("%d", gop),
			"-sc_threshold", "0",
			"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%.0f)", dur),
		)
		if !opts.VideoOnly {
			args = append(args,
				"-c:a", "aac",
				"-ar", "48000",
				"-ac", "2",
				"-profile:a", "aac_low",
				"-af", "aresample=async=1:first_pts=0",
			)
		}
	}

	args = append(args,
		"-t", fmt.Sprintf("%.3f", dur),
		"-avoid_negative_ts", "make_zero",
		"-fflags", "+genpts",
		"-reset_timestamps", "1",
	)
	// Timeline continuity is handled via #EXT-X-DISCONTINUITY in the playlist; each fragment starts near t=0.
	if videoCopyPipeline {
		movFlags := "frag_keyframe+default_base_moof"
		if includeInit {
			movFlags = "empty_moov+" + movFlags
		}
		args = append(args,
			"-movflags", movFlags,
			"-f", "mp4",
			"-loglevel", "warning",
			"-",
		)
	} else {
		// MPEG-TS segments are self-contained; hls.js plays them without #EXT-X-MAP.
		args = append(args,
			"-f", "mpegts",
			"-mpegts_flags", "+initial_discontinuity",
			"-loglevel", "warning",
			"-",
		)
	}

	var buf bytes.Buffer
	if err := runFMP4ToWriter(ctx, runner, &buf, args); err != nil {
		return nil, err
	}
	if buf.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg produced empty segment output")
	}
	return buf.Bytes(), nil
}

// ProbeVideoFPS returns average frame rate from the first video stream, or defaultHLSSegmentFPS.
func ProbeVideoFPS(ctx context.Context, runner *ffexec.Runner, path string) (float64, error) {
	res, err := runner.RunFFprobe(ctx,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=avg_frame_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	if err != nil {
		return defaultHLSSegmentFPS, err
	}
	rate := strings.TrimSpace(res.Stdout)
	if rate == "" || rate == "0/0" {
		return defaultHLSSegmentFPS, nil
	}
	num, den, ok := strings.Cut(rate, "/")
	if !ok {
		return defaultHLSSegmentFPS, nil
	}
	var n, d float64
	if _, err := fmt.Sscanf(num, "%f", &n); err != nil {
		return defaultHLSSegmentFPS, nil
	}
	if _, err := fmt.Sscanf(den, "%f", &d); err != nil || d == 0 {
		return defaultHLSSegmentFPS, nil
	}
	fps := n / d
	if fps <= 0 || fps > 240 {
		return defaultHLSSegmentFPS, nil
	}
	return fps, nil
}

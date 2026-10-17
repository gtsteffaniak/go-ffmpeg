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

// hlsFFmpegLogLevel returns ffmpeg -loglevel for HLS encode jobs. Non-verbose runs use
// "error" to reduce noise; verbose runs follow runner.FFmpegLogLevel() and rely on
// filterMatroskaSeekNoise to drop benign matroska index warnings during input -ss seeks.
func hlsFFmpegLogLevel(r *ffexec.Runner) string {
	if r != nil && r.VerboseFFmpeg {
		return r.FFmpegLogLevel()
	}
	return "error"
}

// appendHLSInputTimestampFlags must appear before -i so demuxers (e.g. AVI) generate PTS for stream copy.
func appendHLSInputTimestampFlags(args []string) []string {
	return append(args, "-fflags", "+genpts")
}

// appendHLSTranscodeCopyTimestampArgs matches Jellyfin HLS transcode timestamp handling.
func appendHLSTranscodeCopyTimestampArgs(args []string) []string {
	return append(args, "-copyts", "-avoid_negative_ts", "disabled")
}

// appendHLSOutputTimestampArgs configures mux output timestamps for HLS fMP4 segments
// when stream-copying without -copyts (legacy video-copy path).
func appendHLSOutputTimestampArgs(args []string, startSec float64) []string {
	args = append(args, "-avoid_negative_ts", "make_zero")
	if startSec > 0 {
		args = append(args, "-output_ts_offset", fmt.Sprintf("%.6f", startSec))
	}
	return args
}

// appendHLSTimestampArgs configures output timestamps for independent segment encodes.
// output_ts_offset places each fragment on a continuous media timeline without
// #EXT-X-DISCONTINUITY (which causes visible backward skips in hls.js).
func appendHLSTimestampArgs(args []string, startSec float64) []string {
	return appendHLSOutputTimestampArgs(args, startSec)
}

// hlsAudioResampleAsyncSamples is the aresample "async" limit in output samples
// (~4s at 48 kHz). The default (1000) only tolerates ~21 ms of A/V drift, which
// leaves short audio runs in fMP4 segments when EAC3/MKV decode lags video.
const hlsAudioResampleAsyncSamples = 192000

// hlsTranscodeAudioFilter normalizes AAC for HLS fMP4 and stretches/compresses audio
// to track the video clock across segment boundaries.
func hlsTranscodeAudioFilter() string {
	return fmt.Sprintf(
		"aformat=sample_rates=48000:channel_layouts=stereo,aresample=async=%d:min_hard_comp=0.100:first_pts=0",
		hlsAudioResampleAsyncSamples,
	)
}

// minHLSSegmentMediaBytes rejects empty fMP4 fragments (moof+mdat shell only).
const minHLSSegmentMediaBytes = 8192

// minHLSTSBytes rejects empty MPEG-TS segments.
const minHLSTSBytes = 4096

type hlsSegmentMuxer int

const (
	hlsSegmentMuxFMP4 hlsSegmentMuxer = iota
	hlsSegmentMuxMPEGTS
)

// HLSSegmentOptions configures on-demand fMP4 HLS segment generation.
type HLSSegmentOptions struct {
	Input    InputSource
	StartSec float64 // input seek position in the source file
	// MediaTimelineSec is the decode timeline position on the HLS playlist (0-based).
	MediaTimelineSec float64
	DurationSec      float64
	Decode           encode.VideoDecodeProfile
	Profile          encode.VideoProfile
	MaxHeight        int
	Remux            bool
	// VideoCopy stream-copies H.264 video and transcodes audio to AAC (e.g. EAC3 in MKV).
	VideoCopy bool
	// AccurateSeek is deprecated: input -ss is frame-accurate when transcoding; the field is ignored.
	AccurateSeek bool
	// InputStartTimeSec is the container start_time offset (see HLSInputSeekSec).
	InputStartTimeSec float64
	GOP               int
	// InputVideoCodec names the source video codec (e.g. "hevc"). On stream-copy
	// pipelines it selects the fMP4 sample-entry tag browsers require — ffmpeg
	// defaults HEVC to hev1, which MSE implementations refuse to decode.
	InputVideoCodec string
	VideoOnly       bool
	Throttle        encode.ThrottleConfig
	// TrackTimescales maps init-segment track IDs to mdhd timescales for tfdt alignment.
	// When empty, HLSInitAndSegment derives timescales from the init segment; callers
	// encoding later segments should pass the same map from segment 0.
	TrackTimescales map[uint32]uint32
}

// HLSSegment generates a self-contained MPEG-TS segment for full re-encode on-demand HLS.
// Each segment is an independent ffmpeg run; players stitch them with #EXT-X-DISCONTINUITY.
func HLSSegment(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSSegmentOptions) ([]byte, error) {
	if hlsUsesVideoCopyPipeline(opts) {
		return nil, fmt.Errorf("HLSSegment is for full transcode only")
	}
	raw, err := runHLSSegmentWithSeekRetry(ctx, runner, caps, opts, false, hlsSegmentMuxMPEGTS)
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
	raw, err := runHLSSegmentWithSeekRetry(ctx, runner, caps, opts, true, hlsSegmentMuxFMP4)
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
	finalizeOpts := opts
	if len(finalizeOpts.TrackTimescales) == 0 {
		finalizeOpts.TrackTimescales = mp4.TrackTimescalesFromInit(init)
	}
	media, err = finalizeHLSSegmentMedia(media, finalizeOpts)
	if err != nil {
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

func finalizeHLSSegmentMedia(media []byte, opts HLSSegmentOptions) ([]byte, error) {
	patched, err := mp4.AlignFragmentToMediaStartWithTimescales(media, opts.MediaTimelineSec, opts.TrackTimescales)
	if err != nil {
		return nil, fmt.Errorf("align segment tfdt at %.3fs: %w", opts.MediaTimelineSec, err)
	}
	return patched, nil
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
	raw, err := runHLSSegmentWithSeekRetry(ctx, runner, caps, opts, false, hlsSegmentMuxFMP4)
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
	media, err = finalizeHLSSegmentMedia(media, opts)
	if err != nil {
		return err
	}
	if err := validateHLSSegmentMedia(media, opts); err != nil {
		return err
	}
	_, err = w.Write(media)
	return err
}

func runHLSSegmentWithSeekRetry(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSSegmentOptions, includeInit bool, mux hlsSegmentMuxer) ([]byte, error) {
	return runHLSSegmentRaw(ctx, runner, caps, opts, includeInit, mux)
}

func hlsUsesVideoCopyPipeline(opts HLSSegmentOptions) bool {
	return opts.Remux || opts.VideoCopy
}

func buildHLSSegmentFFmpegArgs(runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSSegmentOptions, includeInit bool, mux hlsSegmentMuxer) ([]string, error) {
	dur := opts.DurationSec
	if dur <= 0 {
		dur = DefaultHLSSegmentDurationSec
	}
	gop := opts.GOP
	if gop <= 0 {
		gop = HLSSegmentGOP(defaultHLSSegmentFPS, DefaultOnDemandHLSDefaults())
	}

	args := []string{"-hide_banner", "-nostats", "-loglevel", hlsFFmpegLogLevel(runner), "-y"}
	videoCopyPipeline := hlsUsesVideoCopyPipeline(opts)
	if opts.StartSec > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", opts.StartSec))
	}
	if videoCopyPipeline {
		args = appendHLSInputTimestampFlags(args)
	}
	var inputExtra *InputExtraFlags
	if !videoCopyPipeline && opts.Throttle.Enabled {
		inputExtra = &InputExtraFlags{Throttle: &opts.Throttle, Features: caps.FeatureFlags}
	}
	args = appendInputFlags(args, runner, opts.Input, nil, inputExtra)
	if !videoCopyPipeline {
		resolver := encode.NewResolver(caps)
		if initArgs := resolver.VideoToolboxPreInputInit(opts.Decode, opts.Profile); len(initArgs) > 0 {
			args = append(args, initArgs...)
		}
		decodeArgs, err := resolver.VideoDecoderArgs(opts.Decode)
		if err != nil {
			return nil, err
		}
		args = append(args, decodeArgs...)
		args = append(args, "-fflags", "+discardcorrupt+genpts")
	}
	args = append(args, "-i", opts.Input.URL)
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
				"-bsf:a", "aac_adtstoasc",
			)
		}
	default:
		resolver := encode.NewResolver(caps)
		vidArgs, err := resolver.VideoEncoderArgs(opts.Profile)
		if err != nil {
			return nil, err
		}
		filterArgs, err := resolver.VideoFilterArgs(opts.Profile, opts.Decode, opts.MaxHeight)
		if err != nil {
			return nil, err
		}
		args = append(args, filterArgs...)
		args = append(args, vidArgs...)
		encSel, encErr := resolver.ResolveEncoder(opts.Profile)
		if encErr == nil && (opts.Profile.Codec == "" || opts.Profile.Codec == encode.CodecH264) {
			args = encode.AppendH264FMP4CompatArgs(args, encSel.Accel, opts.MaxHeight)
		}
		x264Only := encErr == nil && (encSel.Accel == capabilities.AccelNone || encSel.Accel == "")
		if mux == hlsSegmentMuxFMP4 {
			args = append(args, "-video_track_timescale", "90000")
		}
		args = append(args, "-g", fmt.Sprintf("%d", gop), "-keyint_min", fmt.Sprintf("%d", gop))
		if x264Only {
			args = append(args, "-sc_threshold", "0")
		}
		args = append(args, "-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%.0f)", dur))
		if !opts.VideoOnly {
			args = append(args,
				"-c:a", "aac",
				"-ar", "48000",
				"-ac", "2",
				"-profile:a", "aac_low",
				"-af", hlsTranscodeAudioFilter(),
			)
		}
		args = append(args, "-fps_mode", "cfr")
	}

	if videoCopyPipeline {
		if tag := fmp4CopyVideoTag(opts.InputVideoCodec); tag != "" {
			args = append(args, "-tag:v", tag)
		}
	}
	args = append(args, "-t", fmt.Sprintf("%.3f", dur))
	args = appendHLSOutputTimestampArgs(args, opts.MediaTimelineSec)
	switch mux {
	case hlsSegmentMuxMPEGTS:
		args = append(args, "-f", "mpegts", "-")
	default:
		// empty_moov is required even for media-only segments; without it ffmpeg emits
		// progressive mp4 (moov+mdat) instead of moof+mdat fragments for hls.js MSE.
		movFlags := "empty_moov+frag_keyframe+default_base_moof"
		args = append(args,
			"-movflags", movFlags,
			"-f", "mp4",
			"-",
		)
	}
	_ = includeInit // init+media are always emitted together for on-demand fMP4 today.
	return args, nil
}

func runHLSSegmentRaw(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSSegmentOptions, includeInit bool, mux hlsSegmentMuxer) ([]byte, error) {
	args, err := buildHLSSegmentFFmpegArgs(runner, caps, opts, includeInit, mux)
	if err != nil {
		return nil, err
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
	args := runner.AppendProtocolWhitelistArgs([]string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=avg_frame_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
	})
	args = append(args, path)
	res, err := runner.RunFFprobe(ctx, args...)
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

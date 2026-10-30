package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// RecordMode states the caller's intent for the video track of a recording.
// The library decides the ffmpeg arguments that implement it.
type RecordMode int

const (
	// RecordTranscode re-encodes video with SegmentRecordOptions.Profile. It is the
	// zero value so existing callers keep their behaviour.
	RecordTranscode RecordMode = iota
	// RecordCopy stream-copies video (and audio unless AudioCodec says otherwise).
	// The input is not probed; ffmpeg fails if the codecs cannot be muxed to MP4.
	RecordCopy
	// RecordAuto probes the input and copies video when the source codec is already
	// the target codec and MP4-compatible, otherwise it transcodes. See DecideSegmentPlan.
	RecordAuto
)

// String returns the lower-case mode name used in plan logs.
func (m RecordMode) String() string {
	switch m {
	case RecordCopy:
		return "copy"
	case RecordAuto:
		return "auto"
	default:
		return "transcode"
	}
}

// Audio selection values for SegmentRecordOptions.AudioCodec.
const (
	// AudioAuto (empty string): RecordAuto copies compatible source audio and
	// otherwise uses AAC; RecordTranscode and RecordCopy default to AAC and copy respectively.
	AudioAuto = ""
	// AudioAAC always re-encodes audio to AAC.
	AudioAAC = "aac"
	// AudioCopy always stream-copies the source audio.
	AudioCopy = "copy"
)

// usesIntent reports whether opts asks for the intent path. The zero value (and
// legacy Profile-only use) keeps the original SegmentRecord argv.
func (o SegmentRecordOptions) usesIntent() bool {
	return o.Mode != RecordTranscode || o.AudioCodec != "" || o.AlignSegmentsToClock ||
		o.Fragmented || o.HardwareDecode || o.OnPlan != nil
}

const (
	defaultSegmentPattern = "%Y-%m-%d_%Hh%Mm_seg.mp4"
	defaultAudioBitrate   = "128k"
	secondsPerDay         = 86400
	fragmentedMovFlags    = "movflags=+frag_keyframe+empty_moov+default_base_moof"
)

// SourceFacts is the subset of a probe that the decision logic uses. Probed is
// false when the probe failed or was not run; decisions then stay conservative.
type SourceFacts struct {
	Probed     bool
	HasVideo   bool
	HasAudio   bool
	VideoCodec string
	AudioCodec string
	Width      int
	Height     int
}

// SourceFactsFromProbe converts a probe result into SourceFacts.
func SourceFactsFromProbe(info probe.StreamInfo, err error) SourceFacts {
	if err != nil || !info.IsValid {
		return SourceFacts{}
	}
	return SourceFacts{
		Probed:     true,
		HasVideo:   info.HasVideo,
		HasAudio:   info.HasAudio,
		VideoCodec: strings.ToLower(info.VideoCodec),
		AudioCodec: strings.ToLower(info.AudioCodec),
		Width:      info.Width,
		Height:     info.Height,
	}
}

// mp4VideoCopyable lists video codecs that are routinely muxed to MP4 and played
// back by browsers. VP9 is excluded on purpose: it is valid in MP4 but
// playback support is uneven, so it is transcoded to the target when requested.
var mp4VideoCopyable = map[string]bool{"h264": true, "hevc": true, "av1": true}

// mp4AudioCopyable lists audio codecs that are safe to stream-copy into MP4.
var mp4AudioCopyable = map[string]bool{"aac": true, "mp3": true, "ac3": true, "eac3": true}

// SegmentPlan is the resolved recording plan. It is returned to callers through
// SegmentRecordOptions.OnPlan so they can log what was chosen and why.
type SegmentPlan struct {
	Mode           RecordMode
	Video          string // "copy" or "transcode"
	VideoCodec     string // codec written: source codec (copy) or target codec (transcode)
	SourceCodec    string // probed source video codec (empty when unprobed)
	VideoReason    string
	Audio          string // "copy", "aac" or "none"
	AudioReason    string
	HardwareDecode bool
	SegmentSeconds int
	Aligned        bool
	Fragmented     bool
	Probed         bool
}

// String is a one-line summary suitable for logs.
func (p SegmentPlan) String() string {
	return fmt.Sprintf("mode=%s video=%s(%s): %s audio=%s: %s segments=%ds aligned=%t fragmented=%t hwdecode=%t probed=%t",
		p.Mode, p.Video, p.VideoCodec, p.VideoReason, p.Audio, p.AudioReason,
		p.SegmentSeconds, p.Aligned, p.Fragmented, p.HardwareDecode, p.Probed)
}

// DecideSegmentPlan resolves opts against the probed source. It is pure, so the
// decision table is unit tested without ffmpeg.
//
// Video rules:
//   - Profile.Codec == encode.CodecCopy or Mode == RecordCopy: copy.
//   - Mode == RecordTranscode: transcode to Profile.Codec (H.264 when empty).
//   - Mode == RecordAuto, probe failed: transcode, because the source is unknown.
//   - Mode == RecordAuto, probed: target is Profile.Codec, or the source codec
//     when Profile.Codec is empty and the source is MP4-compatible (H.264 otherwise).
//     Copy only when the source already is the target and is MP4-compatible.
//
// Audio rules (AudioCodec):
//   - AudioAAC: transcode to AAC. AudioCopy: copy. Both skip audio when the source has none.
//   - AudioAuto: RecordCopy copies; RecordTranscode uses AAC; RecordAuto copies when
//     the source codec is MP4-compatible and uses AAC otherwise (or when unprobed).
func DecideSegmentPlan(opts SegmentRecordOptions, src SourceFacts) SegmentPlan {
	mode := opts.Mode
	if opts.Profile.Codec == encode.CodecCopy {
		mode = RecordCopy
	}
	plan := SegmentPlan{
		Mode:           mode,
		SegmentSeconds: segmentSeconds(opts.SegmentDuration),
		Aligned:        opts.AlignSegmentsToClock,
		Fragmented:     opts.Fragmented,
		Probed:         src.Probed,
		SourceCodec:    src.VideoCodec,
	}
	plan.Video, plan.VideoCodec, plan.VideoReason = decideVideo(mode, opts.Profile, src)
	plan.Audio, plan.AudioReason = decideAudio(mode, opts.AudioCodec, src)
	if plan.Video == "transcode" && opts.HardwareDecode && src.Probed && src.VideoCodec != "" {
		plan.HardwareDecode = true
	}
	return plan
}

func decideVideo(mode RecordMode, profile encode.VideoProfile, src SourceFacts) (action, codec, reason string) {
	target := string(profile.Codec)
	switch mode {
	case RecordCopy:
		if src.Probed && src.VideoCodec != "" {
			return "copy", src.VideoCodec, "forced copy"
		}
		return "copy", target, "forced copy (source not probed)"
	case RecordTranscode:
		if target == "" {
			target = string(encode.CodecH264)
		}
		return "transcode", target, "forced transcode"
	}
	// RecordAuto
	if !src.Probed {
		if target == "" {
			target = string(encode.CodecH264)
		}
		return "transcode", target, "input not probed; re-encoding to target"
	}
	if target == "" {
		target = string(encode.CodecH264)
		if mp4VideoCopyable[src.VideoCodec] {
			target = src.VideoCodec
		}
	}
	if src.VideoCodec == target && mp4VideoCopyable[target] {
		return "copy", target, "source is already " + target + " and MP4-compatible"
	}
	return "transcode", target, fmt.Sprintf("source %q needs re-encode to %s", src.VideoCodec, target)
}

func decideAudio(mode RecordMode, pref string, src SourceFacts) (action, reason string) {
	if src.Probed && !src.HasAudio {
		return "none", "source has no audio"
	}
	switch strings.ToLower(pref) {
	case AudioAAC:
		return "aac", "requested AAC"
	case AudioCopy:
		return "copy", "requested copy"
	}
	switch mode {
	case RecordCopy:
		return "copy", "copy mode"
	case RecordTranscode:
		return "aac", "transcode mode default AAC"
	}
	if !src.Probed {
		return "aac", "input not probed; AAC"
	}
	if mp4AudioCopyable[src.AudioCodec] {
		return "copy", "source " + src.AudioCodec + " is MP4-compatible"
	}
	return "aac", fmt.Sprintf("source %q not MP4-compatible; AAC", src.AudioCodec)
}

func segmentSeconds(d time.Duration) int {
	if d == 0 {
		d = time.Hour
	}
	return int(d.Seconds())
}

// validateSegmentOptions checks the option combinations that cannot work.
func validateSegmentOptions(opts SegmentRecordOptions) error {
	secs := segmentSeconds(opts.SegmentDuration)
	if secs <= 0 {
		return fmt.Errorf("segment record: SegmentDuration must be at least one second")
	}
	if opts.AlignSegmentsToClock && secondsPerDay%secs != 0 {
		return fmt.Errorf("segment record: AlignSegmentsToClock needs a SegmentDuration that divides 24h, got %ds", secs)
	}
	switch strings.ToLower(opts.AudioCodec) {
	case AudioAuto, AudioAAC, AudioCopy:
	default:
		return fmt.Errorf("segment record: unsupported AudioCodec %q", opts.AudioCodec)
	}
	return nil
}

// BuildSegmentRecordArgs returns the ffmpeg argv for a resolved plan. It is pure
// (no process is started) so argv policy can be unit tested.
//
// Flags the library deliberately does not emit:
//   - -map 0: explicit selection (best video 0:v:0, optional first audio) avoids
//     data and subtitle streams that the MP4 muxer may reject.
//   - -break_non_keyframes 0: already the ffmpeg default.
//   - -segment_clocktime_wrap_duration: only matters when the segment length does
//     not divide 24h, which AlignSegmentsToClock now rejects.
//   - -loglevel: the runner's configured level applies.
func BuildSegmentRecordArgs(runner *ffexec.Runner, caps *capabilities.Capabilities, opts SegmentRecordOptions, plan SegmentPlan) ([]string, error) {
	if err := validateSegmentOptions(opts); err != nil {
		return nil, err
	}
	pattern := opts.SegmentPattern
	if pattern == "" {
		pattern = defaultSegmentPattern
	}
	listPath := opts.SegmentListPath
	if listPath == "" {
		listPath = filepath.Join(opts.OutputDir, "segments.ffcat")
	}
	resolver := encode.NewResolver(caps)

	args := []string{"-hide_banner", "-nostats", "-fflags", "+genpts", "-correct_ts_overflow", "1"}
	args = appendInputFlags(args, runner, opts.Input, opts.RTSP, nil)
	if plan.HardwareDecode {
		// Best effort: a missing decoder means software decode, not a failed recording.
		if decodeArgs, err := resolver.VideoDecoderArgs(encode.VideoDecodeProfile{Codec: capabilities.VideoCodec(plan.SourceCodec)}); err == nil {
			args = append(args, decodeArgs...)
		}
	}
	args = append(args, opts.ExtraInputArgs...)
	args = append(args, "-i", opts.Input.URL)

	args = append(args, "-map", "0:v:0")
	if plan.Audio != "none" {
		args = append(args, "-map", "0:a:0?")
	}

	switch plan.Video {
	case "copy":
		args = append(args, "-c:v", "copy")
	default:
		vidArgs, err := resolver.VideoEncoderArgs(videoTargetProfile(opts.Profile, plan.VideoCodec))
		if err != nil {
			return nil, err
		}
		args = append(args, vidArgs...)
	}
	switch plan.Audio {
	case "copy":
		args = append(args, "-c:a", "copy")
	case "aac":
		bitrate := opts.AudioBitrate
		if bitrate == "" {
			bitrate = defaultAudioBitrate
		}
		args = append(args, "-c:a", "aac", "-b:a", bitrate)
	}

	args = append(args, "-f", "segment", "-segment_time", fmt.Sprintf("%d", plan.SegmentSeconds))
	if plan.Aligned {
		args = append(args, "-segment_atclocktime", "1")
	}
	args = append(args, "-segment_format", "mp4")
	if plan.Fragmented {
		args = append(args, "-segment_format_options", fragmentedMovFlags)
	}
	if usesStrftime(pattern) {
		// Time-based names. ffmpeg's strftime mode disables %d index expansion, so
		// index-based patterns must not enable it.
		args = append(args, "-strftime", "1")
	}
	args = append(args,
		"-segment_list", listPath,
		"-segment_list_type", "ffconcat",
		// The list is read while recording (health checks, listing), so it must grow live.
		"-segment_list_flags", "+live",
		"-reset_timestamps", "1",
	)
	args = append(args, opts.ExtraOutputArgs...)
	args = append(args, filepath.Join(opts.OutputDir, pattern))
	return args, nil
}

var strftimeConversion = regexp.MustCompile(`%[a-zA-Z]`)

// usesStrftime reports whether a segment pattern contains strftime conversions
// such as %Y or %H. Index-only patterns such as "seg_%03d.mp4" return false.
func usesStrftime(pattern string) bool {
	return strftimeConversion.MatchString(pattern)
}

// videoTargetProfile returns the profile to encode with, honouring the codec the
// plan resolved to. Other fields come from the caller.
func videoTargetProfile(p encode.VideoProfile, codec string) encode.VideoProfile {
	p.Codec = encode.VideoCodec(codec)
	return p
}

// PlanSegmentRecord probes the input when the plan needs it and returns the plan
// without starting ffmpeg. SegmentRecord calls it internally.
func PlanSegmentRecord(ctx context.Context, runner *ffexec.Runner, opts SegmentRecordOptions) (SegmentPlan, error) {
	if err := validateSegmentOptions(opts); err != nil {
		return SegmentPlan{}, err
	}
	var src SourceFacts
	needsProbe := opts.Mode == RecordAuto || opts.HardwareDecode
	if opts.Profile.Codec == encode.CodecCopy {
		needsProbe = false
	}
	if needsProbe {
		info, err := probe.ProbeStream(ctx, runner, probe.ProbeStreamOptions{URL: opts.Input.URL, StreamType: opts.Input.StreamType})
		src = SourceFactsFromProbe(info, err)
		if src.Probed && !src.HasVideo {
			return SegmentPlan{}, fmt.Errorf("segment record: input %q has no video stream", opts.Input.URL)
		}
	}
	return DecideSegmentPlan(opts, src), nil
}

// segmentRecordIntent records input into segmented MP4 files until ctx is cancelled or
// the input ends. It returns after ffmpeg exits.
func segmentRecordIntent(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts SegmentRecordOptions) error {
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return err
	}
	plan, err := PlanSegmentRecord(ctx, runner, opts)
	if err != nil {
		return err
	}
	args, err := BuildSegmentRecordArgs(runner, caps, opts, plan)
	if err != nil {
		return err
	}
	if opts.OnPlan != nil {
		opts.OnPlan(plan)
	}
	_, err = runner.RunFFmpeg(ctx, args...)
	return err
}

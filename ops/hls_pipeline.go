package ops

import (
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// HLSPipelineOptions configures remux/copy/transcode path selection.
type HLSPipelineOptions struct {
	Preset    encode.HLSPreset
	MaxHeight int
}

// NeedsFullVideoTranscode reports whether video must be re-encoded for the preset.
func NeedsFullVideoTranscode(info probe.StreamInfo, opts HLSPipelineOptions) bool {
	switch encode.NormalizeHLSPreset(opts.Preset) {
	case encode.HLSPresetLowLatency, encode.HLSPresetConstrained:
		return true
	}
	if opts.MaxHeight > 0 && info.Height > opts.MaxHeight {
		return true
	}
	return !isH264VideoCodec(info.VideoCodec)
}

// UseVideoCopy selects H.264 stream-copy with audio transcode (quality preset path).
func UseVideoCopy(info probe.StreamInfo, opts HLSPipelineOptions) bool {
	if CanFMP4StreamCopy(info) || !CanH264VideoCopy(info) {
		return false
	}
	return !NeedsFullVideoTranscode(info, opts)
}

// CanFMP4StreamCopy reports whether remux to fMP4 is possible.
func CanFMP4StreamCopy(info probe.StreamInfo) bool {
	if !info.HasVideo {
		return false
	}
	if !isH264VideoCodec(info.VideoCodec) {
		return false
	}
	audio := strings.ToLower(info.AudioCodec)
	return audio == "" || audio == "aac"
}

// CanH264VideoCopy is true when H.264 can be stream-copied and only audio needs transcoding.
func CanH264VideoCopy(info probe.StreamInfo) bool {
	if !info.HasVideo || !isH264VideoCodec(info.VideoCodec) {
		return false
	}
	audio := strings.ToLower(strings.TrimSpace(info.AudioCodec))
	return audio != "" && audio != "aac"
}

func isH264VideoCodec(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "h264", "avc", "avc1":
		return true
	default:
		return strings.Contains(strings.ToLower(name), "h264")
	}
}

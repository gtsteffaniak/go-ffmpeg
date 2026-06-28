package ops

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// BuildHLSSegmentOptions builds segment options for segment index n.
// keyframeTimeline enables accurate seek for transcode segments aligned to keyframes.
// keyframeSeekTimes, when set, pick an input seek at or before the grid point for stream copy.
func BuildHLSSegmentOptions(path string, index int, params HLSSegmentParams, starts, durations []float64, keyframeTimeline bool, keyframeSeekTimes []float64, segmentDurationSec float64) HLSSegmentOptions {
	if segmentDurationSec <= 0 {
		segmentDurationSec = DefaultHLSSegmentDurationSec
	}
	startSec := float64(index) * segmentDurationSec
	durSec := segmentDurationSec
	mediaTimelineSec := startSec
	if index >= 0 && index < len(starts) {
		startSec = starts[index]
		mediaTimelineSec = starts[index]
	}
	if index >= 0 && index < len(durations) {
		durSec = durations[index]
	}
	if len(keyframeSeekTimes) > 0 && (params.Remux || params.VideoCopy) {
		startSec = KeyframeSeekBefore(keyframeSeekTimes, mediaTimelineSec)
	}
	fullTranscode := !params.Remux && !params.VideoCopy
	return HLSSegmentOptions{
		Input:            InputSource{URL: path, StreamType: probe.StreamFile},
		StartSec:         startSec,
		MediaTimelineSec: mediaTimelineSec,
		DurationSec:      durSec,
		Decode:           params.Decode,
		Profile:          params.Profile,
		MaxHeight:        params.MaxHeight,
		Remux:            params.Remux,
		VideoCopy:        params.VideoCopy,
		GOP:              params.GOP,
		AccurateSeek:     fullTranscode && keyframeTimeline,
		Throttle:         encode.ThrottleConfig{Enabled: false},
	}
}

func formatAccel(sel capabilities.EncoderSelection) string {
	if sel.Encoder == "copy" {
		return "copy"
	}
	if sel.Accel == capabilities.AccelNone || sel.Accel == "" {
		return "software"
	}
	return string(sel.Accel)
}

// DescribeHLSSegmentPlan summarizes the encode path and resolved HW/SW codecs for logging.
func DescribeHLSSegmentPlan(resolver *encode.Resolver, params HLSSegmentParams) string {
	if resolver == nil {
		return "path=unavailable"
	}
	if params.Remux {
		return "path=remux encoder=copy decoder=copy hw=none throttle=off"
	}
	if params.VideoCopy {
		return "path=video-copy encoder=copy hw=none throttle=off"
	}

	var parts []string
	parts = append(parts, "path=transcode")

	enc, err := resolver.ResolveEncoder(params.Profile)
	if err != nil {
		parts = append(parts, fmt.Sprintf("encoder=err(%v)", err))
	} else {
		parts = append(parts,
			fmt.Sprintf("encoder=%s", enc.Encoder),
			fmt.Sprintf("encAccel=%s", formatAccel(enc)),
		)
		if enc.Fallback != "" {
			parts = append(parts, fmt.Sprintf("encFallback=%s", enc.Fallback))
		}
	}

	dec, decErr := resolver.ResolveDecoder(params.Decode)
	if decErr != nil {
		parts = append(parts, fmt.Sprintf("decoder=err(%v)", decErr))
	} else if dec.Decoder != "" {
		parts = append(parts, fmt.Sprintf("decoder=%s", dec.Decoder))
		if dec.Accel != capabilities.AccelNone && dec.Accel != "" {
			parts = append(parts, fmt.Sprintf("decAccel=%s", dec.Accel))
		}
	}

	parts = append(parts, "throttle=off")
	return strings.Join(parts, " ")
}

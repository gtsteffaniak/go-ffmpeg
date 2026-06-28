package ops

import (
	"context"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// HLSSegmentParams holds resolved encode/remux settings for one HLS session.
type HLSSegmentParams struct {
	Remux     bool
	VideoCopy bool
	Decode    encode.VideoDecodeProfile
	Profile   encode.VideoProfile
	MaxHeight int
	GOP       int
}

// HLSSegmentBuildInput describes remux/copy/transcode path selection for one file.
type HLSSegmentBuildInput struct {
	Info      probe.StreamInfo
	Preset    encode.HLSPreset
	MaxHeight int
	Remux     bool
	VideoCopy bool
}

// BuildHLSSegmentBuildInput derives remux/copy/transcode flags from stream info and preset.
func BuildHLSSegmentBuildInput(info probe.StreamInfo, preset encode.HLSPreset, maxHeight int) HLSSegmentBuildInput {
	pipeline := HLSPipelineOptions{Preset: preset, MaxHeight: maxHeight}
	fullTranscode := NeedsFullVideoTranscode(info, pipeline)
	remux := CanFMP4StreamCopy(info) && !fullTranscode
	videoCopy := UseVideoCopy(info, pipeline)
	return HLSSegmentBuildInput{
		Info:      info,
		Preset:    preset,
		MaxHeight: maxHeight,
		Remux:     remux,
		VideoCopy: videoCopy,
	}
}

// BuildHLSSegmentParamsFast assembles encode params without probing fps (GOP uses default).
func BuildHLSSegmentParamsFast(in HLSSegmentBuildInput, defaults OnDemandHLSDefaults) HLSSegmentParams {
	defaults = defaults.Normalized()
	params := HLSSegmentParams{
		Remux:     in.Remux,
		VideoCopy: in.VideoCopy,
		MaxHeight: in.MaxHeight,
		GOP:       defaults.DefaultGOP,
	}
	if !in.Remux && !in.VideoCopy {
		params.Decode = encode.HLSDecodeProfileForOnDemand(in.Info)
		params.Profile = encode.HLSVideoProfile(in.Info, in.Preset, in.MaxHeight)
	}
	return params
}

// BuildHLSSegmentParams resolves GOP from fps when probeFPS is true.
func BuildHLSSegmentParams(ctx context.Context, probeFPS func(context.Context, string) (float64, error), path string, in HLSSegmentBuildInput, defaults OnDemandHLSDefaults, doProbeFPS bool) (HLSSegmentParams, error) {
	params := BuildHLSSegmentParamsFast(in, defaults)
	if !doProbeFPS || params.Remux || params.VideoCopy || probeFPS == nil {
		return params, nil
	}
	defaults = defaults.Normalized()
	fps, err := probeFPS(ctx, path)
	if err != nil {
		fps = defaultHLSSegmentFPS
	}
	params.GOP = HLSSegmentGOP(fps, defaults)
	return params, nil
}

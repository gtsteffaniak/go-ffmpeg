package main

import (
	"context"
	"fmt"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func transcodeHLSSegmentParams(ctx context.Context, svc *goffmpeg.Service, path string, info goffmpeg.StreamInfo, accel capabilities.AccelType) (goffmpeg.HLSSegmentParams, error) {
	defaults := onDemandDefaults()
	gop := goffmpeg.HLSSegmentGOP(30, defaults)
	if fps, err := svc.ProbeVideoFPS(ctx, path); err == nil {
		gop = goffmpeg.HLSSegmentGOP(fps, defaults)
	}

	if isLegacyMPEG4Video(info.VideoCodec) {
		profile := encode.VideoProfile{Codec: encode.CodecH264, GOP: gop}
		if accel == capabilities.AccelNone {
			profile.ForceSoftware = true
		} else {
			caps := cachedCapabilities(svc)
			if caps != nil && !caps.CodecEncodeAvailable(capabilities.CodecH264, accel) {
				return goffmpeg.HLSSegmentParams{}, fmt.Errorf("accel %s not available", accel)
			}
			profile.Accel = accel
		}
		return goffmpeg.HLSSegmentParams{
			Remux: false, VideoCopy: false, MaxHeight: 1080, GOP: gop,
			Decode:  encode.VideoDecodeProfile{ForceSoftware: true},
			Profile: profile,
		}, nil
	}

	decode := encode.HLSDecodeProfileForOnDemand(info)
	profile := encode.VideoProfile{
		Codec:   encode.CodecH264,
		Quality: encode.PresetVeryfast,
		GOP:     gop,
	}
	if accel == capabilities.AccelNone {
		profile.ForceSoftware = true
	} else {
		if !encode.IsKnownHLSInputVideoCodec(info.VideoCodec) {
			return goffmpeg.HLSSegmentParams{}, fmt.Errorf("hardware transcode not supported for %s input", info.VideoCodec)
		}
		caps := cachedCapabilities(svc)
		if caps != nil && !caps.CodecEncodeAvailable(capabilities.CodecH264, accel) {
			return goffmpeg.HLSSegmentParams{}, fmt.Errorf("accel %s not available", accel)
		}
		profile.Accel = accel
	}
	decode = encode.MatchHLSTranscodeDecode(profile, decode)

	return goffmpeg.HLSSegmentParams{
		Remux:     false,
		VideoCopy: false,
		MaxHeight: 1080,
		GOP:       gop,
		Decode:    decode,
		Profile:   profile,
	}, nil
}

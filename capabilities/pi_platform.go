package capabilities

import (
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func applyPiPlatformProfile(caps *Capabilities) {
	if caps == nil {
		return
	}
	model := platform.DetectPiModel()
	if model == platform.PiUnknown {
		return
	}
	prof := platform.PiEncodeProfileForModel(model)
	caps.PiProfile = &prof
	if caps.CodecMatrix == nil {
		return
	}
	h264, ok := caps.CodecMatrix[CodecH264]
	if !ok {
		return
	}
	enc := prof.DefaultH264Encoder
	if enc == "" {
		return
	}
	if !caps.EncoderAvailable(enc) && len(h264.Software) > 0 {
		enc = h264.Software[0]
	}
	if caps.EncoderAvailable(enc) {
		sel := EncoderSelection{
			Encoder:  enc,
			Accel:    EncoderAccel(enc),
			Kind:     encoderKind(enc),
			Fallback: firstSoftwareName(caps, CodecH264),
		}
		if sel.Accel == AccelNone && prof.DefaultH264Preset != "" {
			if caps.Platform.Details == nil {
				caps.Platform.Details = map[string]string{}
			}
			caps.Platform.Details["recommended_h264_preset"] = prof.DefaultH264Preset
		}
		h264.Preferred = sel
		caps.CodecMatrix[CodecH264] = h264
	}
}

func firstSoftwareName(caps *Capabilities, codec VideoCodec) string {
	for _, sw := range SoftwareFallback(codec) {
		if caps.EncoderAvailable(sw) {
			return sw
		}
	}
	return ""
}

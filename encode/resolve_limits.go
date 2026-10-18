package encode

import (
	"fmt"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func (r *Resolver) finalizeEncoderSelection(codec capabilities.VideoCodec, profile VideoProfile, sel capabilities.EncoderSelection) (capabilities.EncoderSelection, error) {
	if r == nil || r.Caps == nil || sel.Encoder == "" {
		return sel, nil
	}
	h := profile.OutputHeight
	fps := profile.FrameRate
	if h <= 0 && fps <= 0 {
		return sel, nil
	}
	if !capabilities.EncodeExceedsLimits(sel.Encoder, r.Caps, h, fps) {
		return sel, nil
	}
	reason := capabilities.ExceedsLimitsReason(sel.Encoder, r.Caps, h, fps)
	if profile.Encoder != "" || (profile.Accel != "" && profile.Accel != capabilities.AccelNone) {
		return capabilities.EncoderSelection{}, unavailableProfile(codec, sel.Encoder, capabilities.EncoderAccel(sel.Encoder), reason)
	}
	return r.fallbackAfterRejected(codec, profile, sel.Encoder, reason)
}

func (r *Resolver) fallbackAfterRejected(codec capabilities.VideoCodec, profile VideoProfile, rejectedEncoder, reason string) (capabilities.EncoderSelection, error) {
	// Ordered device chain: try other devices' hierarchy encoders.
	for _, dev := range r.Caps.DeviceChain {
		h := capabilities.HierarchyForGPU(dev.Vendor, r.Caps.HardwarePlatform)
		for _, accel := range h {
			name := capabilities.CodecEncoderMap(codec, accel)
			if name == "" || name == rejectedEncoder {
				continue
			}
			if !r.Caps.EncoderAvailable(name) {
				continue
			}
			if capabilities.EncodeExceedsLimits(name, r.Caps, profile.OutputHeight, profile.FrameRate) {
				continue
			}
			sel := capabilities.EncoderSelection{
				Encoder: name,
				Accel:   accel,
				Kind:    capabilitiesEncoderKind(name),
			}
			if fb := firstSoftwareFallback(r.Caps, codec); fb != "" && r.Caps.AllowSoftware {
				sel.Fallback = fb
			}
			return sel, nil
		}
	}

	if r.Caps.AllowSoftware {
		sw, err := r.resolveSoftware(codec, "")
		if err == nil {
			return sw, nil
		}
	}

	if !r.Caps.AllowSoftware {
		return capabilities.EncoderSelection{}, unavailableProfile(codec, rejectedEncoder, capabilities.EncoderAccel(rejectedEncoder), reason)
	}
	return capabilities.EncoderSelection{}, unavailableProfile(codec, rejectedEncoder, capabilities.EncoderAccel(rejectedEncoder),
		fmt.Sprintf("%s; no fallback encoder available", reason))
}

// ResolveEncoderForGeometry is like ResolveEncoder but requires output height/FPS for limit checks.
func (r *Resolver) ResolveEncoderForGeometry(codec capabilities.VideoCodec, profile VideoProfile) (capabilities.EncoderSelection, error) {
	p := profile
	if p.Codec == "" {
		p.Codec = codec
	}
	return r.ResolveEncoder(p)
}

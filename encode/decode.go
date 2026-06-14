package encode

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// VideoDecodeProfile configures input-side hardware decode.
type VideoDecodeProfile struct {
	Codec capabilities.VideoCodec

	// Accel selects a hardware backend. Zero value uses the detected preferred path.
	// Set to AccelNone (or ForceSoftware) to skip hardware decode arguments.
	Accel capabilities.AccelType

	// Decoder forces a specific decoder capability key (e.g. h264_qsv, hwaccel:vaapi:h264).
	Decoder string

	// ForceSoftware skips hardware decode initialization.
	ForceSoftware bool
}

// ResolveDecoder picks a decoder for profile using cached capabilities.
func (r *Resolver) ResolveDecoder(profile VideoDecodeProfile) (capabilities.DecoderSelection, error) {
	if r == nil || r.Caps == nil {
		return capabilities.DecoderSelection{}, ErrNotDetected
	}
	if profile.ForceSoftware {
		return capabilities.DecoderSelection{Accel: capabilities.AccelNone}, nil
	}

	codec := profile.Codec
	if codec == "" {
		codec = capabilities.CodecH264
	}

	if err := validateDecodeOverrides(profile); err != nil {
		return capabilities.DecoderSelection{}, err
	}

	if profile.Decoder != "" {
		return r.resolveExplicitDecoder(codec, profile.Decoder)
	}

	accel := profile.Accel
	if accel == "" {
		return r.resolvePreferredDecode(codec)
	}
	if accel == capabilities.AccelNone {
		return r.resolveSoftwareDecode(codec, "")
	}
	return r.resolveHardwareDecode(codec, accel)
}

// ValidateVideoDecodeProfile reports whether the profile can decode on this host.
func (r *Resolver) ValidateVideoDecodeProfile(profile VideoDecodeProfile) error {
	if profile.ForceSoftware {
		return nil
	}
	_, err := r.ResolveDecoder(profile)
	return err
}

func validateDecodeOverrides(profile VideoDecodeProfile) error {
	if profile.Decoder == "" {
		return nil
	}
	if profile.ForceSoftware {
		return &ProfileError{
			Codec:   profile.Codec,
			Decoder: profile.Decoder,
			Reasons: []string{"ForceSoftware conflicts with explicit Decoder"},
		}
	}
	if profile.Accel != "" && profile.Accel != capabilities.DecoderAccel(profile.Decoder) {
		return &ProfileError{
			Codec:   profile.Codec,
			Decoder: profile.Decoder,
			Accel:   profile.Accel,
			Reasons: []string{fmt.Sprintf("Accel %q does not match decoder %q", profile.Accel, profile.Decoder)},
		}
	}
	return nil
}

func (r *Resolver) resolvePreferredDecode(codec capabilities.VideoCodec) (capabilities.DecoderSelection, error) {
	support, ok := r.Caps.CodecMatrix[codec]
	if !ok {
		return capabilities.DecoderSelection{}, unavailableDecodeProfile(codec, "", "", "codec not in capability matrix")
	}
	if support.DecodePreferred.Decoder == "" {
		return capabilities.DecoderSelection{}, unavailableDecodeProfile(codec, "", "", "no decoder available for codec")
	}
	return support.DecodePreferred, nil
}

func (r *Resolver) resolveSoftwareDecode(codec capabilities.VideoCodec, prefer string) (capabilities.DecoderSelection, error) {
	names := capabilities.SoftwareDecodeFallback(codec)
	if prefer != "" {
		names = []string{prefer}
	}
	for _, name := range names {
		if r.Caps.DecoderAvailable(name) {
			return capabilities.DecoderSelection{
				Decoder:  name,
				Accel:    capabilities.AccelNone,
				Kind:     "software",
				Fallback: name,
			}, nil
		}
	}
	return capabilities.DecoderSelection{}, unavailableDecodeProfile(codec, "", capabilities.AccelNone, "no software decoder available for codec")
}

func (r *Resolver) resolveHardwareDecode(codec capabilities.VideoCodec, accel capabilities.AccelType) (capabilities.DecoderSelection, error) {
	name := capabilities.CodecDecoderMap(codec, accel)
	if name == "" {
		name = capabilities.CodecHWAccelDecodeKey(codec, accel)
	}
	if name == "" {
		return capabilities.DecoderSelection{}, unavailableDecodeProfile(
			codec, "", accel,
			fmt.Sprintf("acceleration %q is not supported for codec %q", accel, codec),
		)
	}
	if !r.Caps.DecoderAvailable(name) {
		reason := decoderUnavailableReason(r.Caps, name)
		return capabilities.DecoderSelection{}, unavailableDecodeProfile(codec, name, accel, reason)
	}
	sel := capabilities.DecoderSelection{
		Decoder: name,
		Accel:   accel,
		Kind:    capabilities.DecoderKindForName(name),
	}
	if dec, ok := r.Caps.Decoders[name]; ok {
		sel.HWAccel = dec.HWAccel
		sel.SWCodec = dec.SWCodec
	}
	if sw := firstSoftwareDecodeFallback(r.Caps, codec); sw != "" {
		sel.Fallback = sw
	}
	return sel, nil
}

func (r *Resolver) resolveExplicitDecoder(codec capabilities.VideoCodec, decoder string) (capabilities.DecoderSelection, error) {
	if !capabilities.DecoderMatchesCodec(decoder, codec) {
		return capabilities.DecoderSelection{}, unavailableDecodeProfile(
			codec, decoder, capabilities.DecoderAccel(decoder),
			fmt.Sprintf("decoder %q does not match codec %q", decoder, codec),
		)
	}
	if !r.Caps.DecoderAvailable(decoder) {
		reason := decoderUnavailableReason(r.Caps, decoder)
		return capabilities.DecoderSelection{}, unavailableDecodeProfile(codec, decoder, capabilities.DecoderAccel(decoder), reason)
	}
	sel := capabilities.DecoderSelection{
		Decoder: decoder,
		Accel:   capabilities.DecoderAccel(decoder),
		Kind:    capabilities.DecoderKindForName(decoder),
	}
	if dec, ok := r.Caps.Decoders[decoder]; ok {
		sel.HWAccel = dec.HWAccel
		sel.SWCodec = dec.SWCodec
	}
	return sel, nil
}

func firstSoftwareDecodeFallback(caps *capabilities.Capabilities, codec capabilities.VideoCodec) string {
	for _, sw := range capabilities.SoftwareDecodeFallback(codec) {
		if caps.DecoderAvailable(sw) {
			return sw
		}
	}
	return ""
}

func decoderUnavailableReason(caps *capabilities.Capabilities, name string) string {
	dec, ok := caps.Decoders[name]
	if !ok {
		return "decoder not probed"
	}
	if dec.Available {
		return ""
	}
	if dec.TestError != "" {
		return dec.TestError
	}
	if !dec.Compiled {
		return "decoder not compiled in this FFmpeg build"
	}
	return "decoder unavailable"
}

func unavailableDecodeProfile(codec capabilities.VideoCodec, decoder string, accel capabilities.AccelType, reason string) *ProfileError {
	reasons := []string{reason}
	if reason == "" {
		reasons = []string{"decoder unavailable"}
	}
	return &ProfileError{Codec: codec, Decoder: decoder, Accel: accel, Reasons: reasons}
}

// VideoDecoderArgs returns ffmpeg arguments placed before -i for hardware decode.
func (r *Resolver) VideoDecoderArgs(profile VideoDecodeProfile) ([]string, error) {
	if profile.ForceSoftware || r.Caps == nil {
		return nil, nil
	}

	sel, err := r.ResolveDecoder(profile)
	if err != nil {
		return nil, err
	}
	if sel.Accel == capabilities.AccelNone || sel.Decoder == "" {
		return nil, nil
	}

	switch sel.Accel {
	case capabilities.AccelNVENC:
		return []string{"-c:v", sel.Decoder}, nil
	case capabilities.AccelQSV:
		return []string{"-init_hw_device", "qsv=hw", "-c:v", sel.Decoder}, nil
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		renderDev := platform.RenderDevice(r.Caps.Platform.Details)
		sw := sel.SWCodec
		if sw == "" {
			sw = softwareCodecForDecode(sel.Decoder)
		}
		if sw == "" {
			return nil, unavailableDecodeProfile(profile.Codec, sel.Decoder, sel.Accel, "no software codec for vaapi decode path")
		}
		return []string{
			"-init_hw_device", "vaapi=va:" + renderDev,
			"-hwaccel", "vaapi",
			"-hwaccel_device", "va",
			"-c:v", sw,
		}, nil
	default:
		return nil, nil
	}
}

func softwareCodecForDecode(decoder string) string {
	switch {
	case strings.Contains(decoder, "h264"):
		return "h264"
	case strings.Contains(decoder, "hevc"):
		return "hevc"
	case strings.Contains(decoder, "vp9"):
		return "vp9"
	case strings.Contains(decoder, "av1"):
		return "av1"
	default:
		return decoder
	}
}

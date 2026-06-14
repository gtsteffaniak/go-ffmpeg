package encode

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

// ResolveEncoder picks an encoder for profile using cached capabilities.
// Zero-value Accel means automatic (preferred path). AccelNone means software.
// Non-empty Encoder forces that ffmpeg encoder when available.
func (r *Resolver) ResolveEncoder(profile VideoProfile) (capabilities.EncoderSelection, error) {
	if r == nil || r.Caps == nil {
		return capabilities.EncoderSelection{}, ErrNotDetected
	}
	if profile.Codec == CodecCopy {
		return capabilities.EncoderSelection{Encoder: "copy", Kind: "copy"}, nil
	}

	codec := profile.Codec
	if codec == "" {
		codec = CodecH264
	}

	if err := validateProfileOverrides(profile); err != nil {
		return capabilities.EncoderSelection{}, err
	}

	if profile.Encoder != "" {
		return r.resolveExplicitEncoder(codec, profile.Encoder)
	}

	accel := profile.Accel
	if profile.ForceSoftware {
		accel = capabilities.AccelNone
	}

	if accel == "" {
		return r.resolvePreferred(codec)
	}
	if accel == capabilities.AccelNone {
		return r.resolveSoftware(codec, "")
	}
	return r.resolveHardware(codec, accel)
}

// ValidateVideoProfile reports whether the profile can be encoded on this host.
func (r *Resolver) ValidateVideoProfile(profile VideoProfile) error {
	_, err := r.ResolveEncoder(profile)
	return err
}

func validateProfileOverrides(profile VideoProfile) error {
	if profile.Encoder == "" {
		return nil
	}
	if profile.ForceSoftware {
		return &ProfileError{
			Codec:   profile.Codec,
			Encoder: profile.Encoder,
			Reasons: []string{"ForceSoftware conflicts with explicit Encoder"},
		}
	}
	if profile.Accel != "" && profile.Accel != capabilities.EncoderAccel(profile.Encoder) {
		return &ProfileError{
			Codec:   profile.Codec,
			Encoder: profile.Encoder,
			Accel:   profile.Accel,
			Reasons: []string{fmt.Sprintf("Accel %q does not match encoder %q", profile.Accel, profile.Encoder)},
		}
	}
	return nil
}

func (r *Resolver) resolvePreferred(codec capabilities.VideoCodec) (capabilities.EncoderSelection, error) {
	support, ok := r.Caps.CodecMatrix[codec]
	if !ok {
		return capabilities.EncoderSelection{}, unavailableProfile(codec, "", "", "codec not in capability matrix")
	}
	if support.Preferred.Encoder == "" {
		return capabilities.EncoderSelection{}, unavailableProfile(codec, "", "", "no encoder available for codec")
	}
	return support.Preferred, nil
}

func (r *Resolver) resolveSoftware(codec capabilities.VideoCodec, prefer string) (capabilities.EncoderSelection, error) {
	fallbacks := capabilities.SoftwareFallback(codec)
	if prefer != "" {
		fallbacks = []string{prefer}
	}
	for _, name := range fallbacks {
		if !capabilities.EncoderMatchesCodec(name, codec) {
			continue
		}
		if r.Caps.EncoderAvailable(name) {
			return capabilities.EncoderSelection{
				Encoder: name,
				Accel:   capabilities.AccelNone,
				Kind:    capabilitiesEncoderKind(name),
			}, nil
		}
	}
	reason := softwareUnavailableReason(r.Caps, codec, fallbacks)
	return capabilities.EncoderSelection{}, unavailableProfile(codec, "", capabilities.AccelNone, reason)
}

func (r *Resolver) resolveHardware(codec capabilities.VideoCodec, accel capabilities.AccelType) (capabilities.EncoderSelection, error) {
	name := capabilities.CodecEncoderMap(codec, accel)
	if name == "" {
		return capabilities.EncoderSelection{}, unavailableProfile(
			codec, "", accel,
			fmt.Sprintf("acceleration %q is not supported for codec %q", accel, codec),
		)
	}
	if !r.Caps.EncoderAvailable(name) {
		reason := encoderUnavailableReason(r.Caps, name)
		return capabilities.EncoderSelection{}, unavailableProfile(codec, name, accel, reason)
	}
	return capabilities.EncoderSelection{
		Encoder:  name,
		Accel:    accel,
		Kind:     capabilitiesEncoderKind(name),
		Fallback: firstSoftwareFallback(r.Caps, codec),
	}, nil
}

func (r *Resolver) resolveExplicitEncoder(codec capabilities.VideoCodec, encoder string) (capabilities.EncoderSelection, error) {
	if !capabilities.EncoderMatchesCodec(encoder, codec) {
		return capabilities.EncoderSelection{}, unavailableProfile(
			codec, encoder, capabilities.EncoderAccel(encoder),
			fmt.Sprintf("encoder %q does not match codec %q", encoder, codec),
		)
	}
	if !r.Caps.EncoderAvailable(encoder) {
		reason := encoderUnavailableReason(r.Caps, encoder)
		return capabilities.EncoderSelection{}, unavailableProfile(codec, encoder, capabilities.EncoderAccel(encoder), reason)
	}
	return capabilities.EncoderSelection{
		Encoder:  encoder,
		Accel:    capabilities.EncoderAccel(encoder),
		Kind:     capabilitiesEncoderKind(encoder),
		Fallback: firstSoftwareFallback(r.Caps, codec),
	}, nil
}

func firstSoftwareFallback(caps *capabilities.Capabilities, codec capabilities.VideoCodec) string {
	for _, sw := range capabilities.SoftwareFallback(codec) {
		if caps.EncoderAvailable(sw) {
			return sw
		}
	}
	return ""
}

func softwareUnavailableReason(caps *capabilities.Capabilities, codec capabilities.VideoCodec, names []string) string {
	var reasons []string
	for _, name := range names {
		if reason := encoderUnavailableReason(caps, name); reason != "" {
			reasons = append(reasons, name+": "+reason)
		}
	}
	if len(reasons) == 0 {
		return "no software encoder available for codec"
	}
	return strings.Join(reasons, "; ")
}

func encoderUnavailableReason(caps *capabilities.Capabilities, name string) string {
	enc, ok := caps.Encoders[name]
	if !ok {
		return "encoder not probed"
	}
	if enc.Available {
		return ""
	}
	if enc.TestError != "" {
		return enc.TestError
	}
	if !enc.Compiled {
		return "encoder not compiled in this FFmpeg build"
	}
	return "encoder unavailable"
}

func unavailableProfile(codec capabilities.VideoCodec, encoder string, accel capabilities.AccelType, reason string) *ProfileError {
	reasons := []string{reason}
	if reason == "" {
		reasons = []string{"encoder unavailable"}
	}
	return &ProfileError{Codec: codec, Encoder: encoder, Accel: accel, Reasons: reasons}
}

func capabilitiesEncoderKind(name string) string {
	for _, e := range capabilities.KnownEncoders {
		if e.Name == name {
			return e.Kind
		}
	}
	return "unknown"
}

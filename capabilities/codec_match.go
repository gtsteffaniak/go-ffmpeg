package capabilities

// EncoderMatchesCodec reports whether an ffmpeg encoder name is valid for a codec family.
func EncoderMatchesCodec(encoder string, codec VideoCodec) bool {
	for _, sw := range SoftwareFallback(codec) {
		if sw == encoder {
			return true
		}
	}
	for _, accel := range AllEncoderAccelTypes() {
		if CodecEncoderMap(codec, accel) == encoder {
			return true
		}
	}
	return false
}

// DecoderMatchesCodec reports whether a decoder capability key is valid for a codec family.
func DecoderMatchesCodec(decoder string, codec VideoCodec) bool {
	for _, sw := range SoftwareDecodeFallback(codec) {
		if sw == decoder {
			return true
		}
	}
	for _, accel := range AllEncoderAccelTypes() {
		if name := CodecDecoderMap(codec, accel); name == decoder {
			return true
		}
		if name := CodecHWAccelDecodeKey(codec, accel); name == decoder {
			return true
		}
	}
	return false
}

// EncoderAccel infers the acceleration backend for a known encoder name.
func EncoderAccel(encoder string) AccelType {
	for _, e := range KnownEncoders {
		if e.Name != encoder {
			continue
		}
		switch e.Kind {
		case "nvenc":
			return AccelNVENC
		case "amf":
			return AccelAMF
		case "qsv":
			return AccelQSV
		case "vaapi":
			return AccelVAAPI
		case "videotoolbox":
			return AccelVideoToolbox
		default:
			return AccelNone
		}
	}
	return AccelNone
}

// DecoderAccel infers the acceleration backend for a known decoder name.
func DecoderAccel(decoder string) AccelType {
	for _, d := range KnownDecoders {
		if d.Name != decoder {
			continue
		}
		switch d.Kind {
		case "nvenc":
			return AccelNVENC
		case "qsv":
			return AccelQSV
		case "vaapi":
			return AccelVAAPI
		case "videotoolbox":
			return AccelVideoToolbox
		default:
			return AccelNone
		}
	}
	if decoderKindForName(decoder) == "vaapi" {
		return AccelVAAPI
	}
	if decoderKindForName(decoder) == "videotoolbox" {
		return AccelVideoToolbox
	}
	return AccelNone
}

package capabilities

// EncodeOption describes one encode path for a codec family (built at detect time).
type EncodeOption struct {
	Codec             VideoCodec `json:"codec"`
	Encoder           string     `json:"encoder"`
	Accel             AccelType  `json:"accel"`
	Kind              string     `json:"kind"`
	Label             string     `json:"label"`
	Available         bool       `json:"available"`
	Default           bool       `json:"default"`
	UnavailableReason string     `json:"unavailableReason,omitempty"`
}

// DecodeOption describes one decode path for a codec family (built at detect time).
type DecodeOption struct {
	Codec             VideoCodec `json:"codec"`
	Decoder           string     `json:"decoder"`
	Accel             AccelType  `json:"accel"`
	Kind              string     `json:"kind"`
	Label             string     `json:"label"`
	Available         bool       `json:"available"`
	Default           bool       `json:"default"`
	UnavailableReason string     `json:"unavailableReason,omitempty"`
}

// optionAccels lists every backend an option list should cover: the selection
// hierarchy first, then the remaining backends the platform could use. A
// software-only or single-device selection must not hide the other backends.
func optionAccels(caps *Capabilities) []AccelType {
	out := append([]AccelType(nil), caps.EncoderHierarchy...)
	if len(out) == 0 {
		out = HierarchyForPlatform(caps.Platform)
	} else {
		have := map[AccelType]bool{}
		for _, a := range out {
			have[a] = true
		}
		for _, a := range HierarchyForPlatform(caps.Platform) {
			if !have[a] {
				out = append(out, a)
			}
		}
	}
	return out
}

// BuildEncodeOptions fills EncodeOptions from the codec matrix and encoder catalog.
func BuildEncodeOptions(caps *Capabilities) {
	if caps == nil {
		return
	}
	var out []EncodeOption
	codecs := []VideoCodec{CodecH264, CodecHEVC, CodecAV1, CodecVP9}
	hierarchy := optionAccels(caps)
	for _, codec := range codecs {
		support, ok := caps.CodecMatrix[codec]
		if !ok {
			continue
		}
		preferred := support.Preferred.Encoder
		seen := map[string]struct{}{}
		for _, accel := range hierarchy {
			name := CodecEncoderMap(codec, accel)
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, encodeOptionFromName(caps, codec, name, accel, name == preferred))
		}
		for _, name := range SoftwareFallback(codec) {
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, encodeOptionFromName(caps, codec, name, AccelNone, name == preferred))
		}
	}
	caps.EncodeOptions = out
}

// BuildDecodeOptions fills DecodeOptions from the codec matrix and decoder catalog.
func BuildDecodeOptions(caps *Capabilities) {
	if caps == nil {
		return
	}
	var out []DecodeOption
	codecs := []VideoCodec{CodecH264, CodecHEVC, CodecAV1, CodecVP9}
	hierarchy := optionAccels(caps)
	for _, codec := range codecs {
		support, ok := caps.CodecMatrix[codec]
		if !ok {
			continue
		}
		preferred := support.DecodePreferred.Decoder
		seen := map[string]struct{}{}
		for _, accel := range hierarchy {
			name := CodecDecoderMap(codec, accel)
			if name == "" {
				name = CodecHWAccelDecodeKey(codec, accel)
			}
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, decodeOptionFromName(caps, codec, name, accel, name == preferred))
		}
		for _, name := range SoftwareDecodeFallback(codec) {
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, decodeOptionFromName(caps, codec, name, AccelNone, name == preferred))
		}
	}
	caps.DecodeOptions = out
}

func encodeOptionFromName(caps *Capabilities, codec VideoCodec, name string, accel AccelType, isDefault bool) EncodeOption {
	kind := encoderKind(name)
	opt := EncodeOption{
		Codec:     codec,
		Encoder:   name,
		Accel:     accel,
		Kind:      kind,
		Label:     EncoderLabel(name),
		Default:   isDefault,
		Available: caps.EncoderAvailable(name),
	}
	if enc, ok := caps.Encoders[name]; ok && !enc.Available {
		opt.UnavailableReason = enc.TestError
		if opt.UnavailableReason == "" && enc.ProbeHint != "" {
			opt.UnavailableReason = formatProbeError(enc.ProbeCode, enc.ProbeHint, enc.ProbeDetail)
		}
		if opt.UnavailableReason == "" && !enc.Compiled {
			opt.UnavailableReason = "encoder not compiled in this FFmpeg build"
		}
	}
	return opt
}

func decodeOptionFromName(caps *Capabilities, codec VideoCodec, name string, accel AccelType, isDefault bool) DecodeOption {
	opt := DecodeOption{
		Codec:     codec,
		Decoder:   name,
		Accel:     accel,
		Kind:      decoderKindForName(name),
		Label:     DecoderLabel(name),
		Default:   isDefault,
		Available: caps.DecoderAvailable(name),
	}
	if dec, ok := caps.Decoders[name]; ok && !dec.Available {
		opt.UnavailableReason = dec.TestError
		if opt.UnavailableReason == "" && !dec.Compiled {
			opt.UnavailableReason = "decoder not compiled in this FFmpeg build"
		}
	}
	return opt
}

// AvailableEncodeOptions returns encode options that passed detection.
func (c *Capabilities) AvailableEncodeOptions() []EncodeOption {
	if c == nil {
		return nil
	}
	var out []EncodeOption
	for _, opt := range c.EncodeOptions {
		if opt.Available {
			out = append(out, opt)
		}
	}
	return out
}

// AvailableDecodeOptions returns decode options that passed detection.
func (c *Capabilities) AvailableDecodeOptions() []DecodeOption {
	if c == nil {
		return nil
	}
	var out []DecodeOption
	for _, opt := range c.DecodeOptions {
		if opt.Available {
			out = append(out, opt)
		}
	}
	return out
}

// EncodeOptionsForCodec returns all encode options for one codec family.
func (c *Capabilities) EncodeOptionsForCodec(codec VideoCodec) []EncodeOption {
	if c == nil {
		return nil
	}
	var out []EncodeOption
	for _, opt := range c.EncodeOptions {
		if opt.Codec == codec {
			out = append(out, opt)
		}
	}
	return out
}

// DecodeOptionsForCodec returns all decode options for one codec family.
func (c *Capabilities) DecodeOptionsForCodec(codec VideoCodec) []DecodeOption {
	if c == nil {
		return nil
	}
	var out []DecodeOption
	for _, opt := range c.DecodeOptions {
		if opt.Codec == codec {
			out = append(out, opt)
		}
	}
	return out
}

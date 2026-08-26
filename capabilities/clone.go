package capabilities

// Clone returns a deep copy of the capability matrix safe for callers to read
// without mutating the Service's internal state.
func (c *Capabilities) Clone() *Capabilities {
	if c == nil {
		return nil
	}
	out := *c
	out.Encoders = cloneEncoderMap(c.Encoders)
	out.Decoders = cloneDecoderMap(c.Decoders)
	out.Filters = cloneStringBoolMap(c.Filters)
	out.HWAccels = cloneHWAccelMap(c.HWAccels)
	out.Protocols = cloneStringBoolMap(c.Protocols)
	out.CodecMatrix = cloneCodecMatrix(c.CodecMatrix)
	out.EncodeOptions = append([]EncodeOption(nil), c.EncodeOptions...)
	out.DecodeOptions = append([]DecodeOption(nil), c.DecodeOptions...)
	out.EnabledOps = append([]string(nil), c.EnabledOps...)
	out.DisabledOps = cloneDisabledOps(c.DisabledOps)
	out.EncoderHierarchy = append([]AccelType(nil), c.EncoderHierarchy...)
	if c.SelectedGPU != nil {
		gpu := *c.SelectedGPU
		out.SelectedGPU = &gpu
	}
	out.Platform = c.Platform
	if c.Platform.Details != nil {
		out.Platform.Details = cloneStringStringMap(c.Platform.Details)
	}
	out.BuildConfig = c.BuildConfig
	if c.BuildConfig.Flags != nil {
		out.BuildConfig.Flags = append([]string(nil), c.BuildConfig.Flags...)
	}
	if c.BuildConfig.LibFlags != nil {
		out.BuildConfig.LibFlags = append([]string(nil), c.BuildConfig.LibFlags...)
	}
	return &out
}

func cloneEncoderMap(in map[string]EncoderCapability) map[string]EncoderCapability {
	if in == nil {
		return nil
	}
	out := make(map[string]EncoderCapability, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneDecoderMap(in map[string]DecoderCapability) map[string]DecoderCapability {
	if in == nil {
		return nil
	}
	out := make(map[string]DecoderCapability, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneHWAccelMap(in map[string]HWAccelCapability) map[string]HWAccelCapability {
	if in == nil {
		return nil
	}
	out := make(map[string]HWAccelCapability, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneDisabledOps(in map[string][]string) map[string][]string {
	if in == nil {
		return nil
	}
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func cloneCodecMatrix(in map[VideoCodec]CodecSupport) map[VideoCodec]CodecSupport {
	if in == nil {
		return nil
	}
	out := make(map[VideoCodec]CodecSupport, len(in))
	for codec, support := range in {
		copied := support
		copied.Software = append([]string(nil), support.Software...)
		copied.SoftwareDecode = append([]string(nil), support.SoftwareDecode...)
		if support.Hardware != nil {
			copied.Hardware = make(map[AccelType]string, len(support.Hardware))
			for k, v := range support.Hardware {
				copied.Hardware[k] = v
			}
		}
		if support.HardwareDecode != nil {
			copied.HardwareDecode = make(map[AccelType]string, len(support.HardwareDecode))
			for k, v := range support.HardwareDecode {
				copied.HardwareDecode[k] = v
			}
		}
		out[codec] = copied
	}
	return out
}

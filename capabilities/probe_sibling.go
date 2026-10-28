package capabilities

// demoteSiblingProbeFailures re-labels leftover driver-style probe failures as
// [HWProbeCodecUnsupported] when another encoder on the same backend passed.
//
// A backend with at least one working encoder has a working driver stack by
// definition (device opened, driver loaded, session created), so a remaining
// failure for another codec is a hardware/codec limit, not a missing package.
// Without this, an unrecognised ffmpeg error for a codec such as AV1 on Gen12
// was promoted to gpu_present_driver_missing and told the operator to reinstall
// drivers that were already working.
func demoteSiblingProbeFailures(caps *Capabilities) {
	if caps == nil {
		return
	}
	working := map[string]bool{}
	for _, known := range KnownEncoders {
		if !known.HW {
			continue
		}
		if enc, ok := caps.Encoders[known.Name]; ok && enc.Tested && enc.Available {
			working[known.Kind] = true
		}
	}
	for _, known := range KnownEncoders {
		if !known.HW || !working[known.Kind] {
			continue
		}
		enc, ok := caps.Encoders[known.Name]
		if !ok || !enc.Tested || enc.Available {
			continue
		}
		if enc.ProbeCode != HWProbeUnknown && enc.ProbeCode != HWProbeGPUPresentDriverMissing {
			continue
		}
		hint := HintForProbeCode(HWProbeCodecUnsupported)
		enc.ProbeCode = HWProbeCodecUnsupported
		enc.ProbeHint = hint
		enc.TestError = formatProbeError(HWProbeCodecUnsupported, hint, enc.ProbeDetail)
		caps.Encoders[known.Name] = enc
		caps.refreshEncoderProbeIndex(EncoderProbeResult{
			Encoder: known.Name,
			Accel:   accelForEncoderKind(known.Kind),
			Code:    HWProbeCodecUnsupported,
			Hint:    hint,
			Detail:  enc.ProbeDetail,
		})
	}
}

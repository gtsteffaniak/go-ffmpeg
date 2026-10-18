package capabilities

// EncodeLimits describes known output constraints for a hardware encoder.
type EncodeLimits struct {
	MaxHeight int
	MaxFPS    float64
}

// EncodeLimitsFor returns runtime limits for an encoder (0 = no limit on that axis).
func EncodeLimitsFor(encoder string, caps *Capabilities) EncodeLimits {
	if caps != nil && caps.PiProfile != nil {
		switch encoder {
		case "h264_v4l2m2m":
			lim := EncodeLimits{MaxFPS: float64(caps.PiProfile.RecommendedMaxFPS)}
			if caps.PiProfile.MaxHWEncodeHeight > 0 {
				lim.MaxHeight = caps.PiProfile.MaxHWEncodeHeight
			} else if caps.PiProfile.RecommendedMaxHeight > 0 {
				lim.MaxHeight = caps.PiProfile.RecommendedMaxHeight
			}
			if lim.MaxHeight == 0 {
				lim.MaxHeight = 1080
			}
			if lim.MaxFPS == 0 {
				lim.MaxFPS = 30
			}
			return lim
		}
	}
	switch encoder {
	case "h264_v4l2m2m":
		return EncodeLimits{MaxHeight: 1080, MaxFPS: 30}
	default:
		return EncodeLimits{}
	}
}

// EncodeExceedsLimits reports whether output height/FPS exceeds encoder limits.
// Zero height or fps skips that axis (unknown stream geometry).
func EncodeExceedsLimits(encoder string, caps *Capabilities, outputHeight int, fps float64) bool {
	lim := EncodeLimitsFor(encoder, caps)
	if lim.MaxHeight > 0 && outputHeight > 0 && outputHeight > lim.MaxHeight {
		return true
	}
	if lim.MaxFPS > 0 && fps > 0 && fps > lim.MaxFPS {
		return true
	}
	return false
}

// ExceedsLimitsReason returns a human-readable reason when limits are exceeded.
func ExceedsLimitsReason(encoder string, caps *Capabilities, outputHeight int, fps float64) string {
	lim := EncodeLimitsFor(encoder, caps)
	if lim.MaxHeight > 0 && outputHeight > 0 && outputHeight > lim.MaxHeight {
		return fmtLimit(encoder, outputHeight, lim.MaxHeight, "height")
	}
	if lim.MaxFPS > 0 && fps > 0 && fps > lim.MaxFPS {
		return fmtLimitFPS(encoder, fps, lim.MaxFPS)
	}
	return ""
}

func fmtLimit(encoder string, got, max int, axis string) string {
	return encoder + ": " + axis + " " + itoa(got) + " exceeds hardware max " + itoa(max)
}

func fmtLimitFPS(encoder string, got, max float64) string {
	return encoder + ": fps exceeds hardware max"
}

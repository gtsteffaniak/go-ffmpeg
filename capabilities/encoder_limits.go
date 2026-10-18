package capabilities

import "github.com/gtsteffaniak/go-ffmpeg/platform"

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
			if caps.PiProfile.Model == platform.Pi4 {
				// Pi 4 uses resolution/FPS combinations; see pi4HWEncodeExceeds.
				return EncodeLimits{}
			}
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
	return EncodeLimits{}
}

// EncodeExceedsLimits reports whether output height/FPS exceeds encoder limits.
// Zero height or fps skips that axis (unknown stream geometry).
func EncodeExceedsLimits(encoder string, caps *Capabilities, outputHeight int, fps float64) bool {
	if caps != nil && caps.PiProfile != nil && caps.PiProfile.Model == platform.Pi4 && encoder == "h264_v4l2m2m" {
		return pi4HWEncodeExceeds(outputHeight, fps)
	}
	lim := EncodeLimitsFor(encoder, caps)
	if lim.MaxHeight > 0 && outputHeight > 0 && outputHeight > lim.MaxHeight {
		return true
	}
	if lim.MaxFPS > 0 && fps > 0 && fps > lim.MaxFPS {
		return true
	}
	return false
}

// pi4HWEncodeExceeds applies Pi 4 h264_v4l2m2m mode limits (1080p30, 720p60).
func pi4HWEncodeExceeds(outputHeight int, fps float64) bool {
	if outputHeight > 0 && outputHeight > 1080 {
		return true
	}
	if fps <= 0 {
		return false
	}
	if outputHeight > 0 {
		if outputHeight <= 720 {
			return fps > 60
		}
		return fps > 30
	}
	return fps > 60
}

// ExceedsLimitsReason returns a human-readable reason when limits are exceeded.
func ExceedsLimitsReason(encoder string, caps *Capabilities, outputHeight int, fps float64) string {
	if caps != nil && caps.PiProfile != nil && caps.PiProfile.Model == platform.Pi4 && encoder == "h264_v4l2m2m" {
		if pi4HWEncodeExceeds(outputHeight, fps) {
			return encoder + ": output exceeds Pi 4 hardware encode limits (1080p30 or 720p60)"
		}
		return ""
	}
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

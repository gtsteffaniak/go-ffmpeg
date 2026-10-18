package capabilities

import (
	"fmt"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// ApplyDeviceSelection rewrites codec preferred paths using an ordered device chain.
// When chain is empty, selection follows EncoderHierarchy on the active GPU choice only.
func ApplyDeviceSelection(caps *Capabilities, hierarchy []AccelType, chain []platform.GPUChoice, active platform.GPUChoice) {
	if caps == nil {
		return
	}
	devices := chain
	if len(devices) == 0 && active.Enabled && !active.SoftwareOnly {
		devices = []platform.GPUChoice{active}
	}
	if len(devices) == 0 {
		return
	}
	codecs := []VideoCodec{CodecH264, CodecHEVC, CodecAV1, CodecVP9}
	for _, codec := range codecs {
		if caps.PiProfile != nil && codec == CodecH264 && caps.PiProfile.DefaultH264Encoder != "" {
			continue // Pi profile already pinned mandatory H.264 path
		}
		sel, attempts := selectEncodeForDeviceChain(caps, codec, devices, hierarchy)
		caps.DeviceAttempts = append(caps.DeviceAttempts, attempts...)
		if sel.Encoder == "" {
			if !caps.AllowSoftware {
				continue
			}
			support := caps.CodecMatrix[codec]
			sel = resolvePreferredEncode(support, append(hierarchy, AccelNone))
		}
		if sel.Encoder != "" {
			support, ok := caps.CodecMatrix[codec]
			if ok {
				support.Preferred = sel
				caps.CodecMatrix[codec] = support
			}
		}
	}
}

func selectEncodeForDeviceChain(caps *Capabilities, codec VideoCodec, devices []platform.GPUChoice, globalHierarchy []AccelType) (EncoderSelection, []DeviceCodecAttempt) {
	var attempts []DeviceCodecAttempt
	for _, dev := range devices {
		h := HierarchyForGPU(dev.Vendor, caps.HardwarePlatform)
		if len(h) == 0 {
			h = globalHierarchy
		}
		for _, accel := range h {
			name := CodecEncoderMap(codec, accel)
			if name == "" {
				continue
			}
			att := DeviceCodecAttempt{Codec: codec, Device: deviceLabel(dev), Encoder: name}
			if !caps.EncoderAvailable(name) {
				if enc, ok := caps.Encoders[name]; ok && enc.TestError != "" {
					att.Reason = enc.TestError
				} else if enc.SelectionNote != "" {
					att.Reason = enc.SelectionNote
				} else {
					att.Reason = "encoder unavailable"
				}
				attempts = append(attempts, att)
				continue
			}
			att.Selected = true
			attempts = append(attempts, att)
			sel := EncoderSelection{Encoder: name, Accel: accel, Kind: encoderKind(name)}
			if fb := firstSoftwareName(caps, codec); fb != "" && caps.AllowSoftware {
				sel.Fallback = fb
			}
			return sel, attempts
		}
		attempts = append(attempts, DeviceCodecAttempt{
			Codec:  codec,
			Device: deviceLabel(dev),
			Reason: fmt.Sprintf("no working encoder for %s on device", codec),
		})
	}
	return EncoderSelection{}, attempts
}

func deviceLabel(c platform.GPUChoice) string {
	if c.Name != "" {
		return c.Name
	}
	if c.Label != "" {
		return c.Label
	}
	return string(c.Mode)
}

package capabilities

import (
	"errors"
	"fmt"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

var errNoHardwareEncoder = errors.New("no hardware encoder available on selected device chain (software fallback disabled)")

// ApplyDeviceSelection rewrites codec preferred paths using an ordered device chain.
// When chain is empty, selection follows EncoderHierarchy on the active GPU choice only.
func ApplyDeviceSelection(caps *Capabilities, hierarchy []AccelType, chain []platform.GPUChoice, active platform.GPUChoice) error {
	if caps == nil {
		return nil
	}
	devices := chain
	if len(devices) == 0 && active.Enabled && !active.SoftwareOnly {
		devices = []platform.GPUChoice{active}
	}
	if len(devices) == 0 {
		return nil
	}
	var hardwareSelected bool
	codecs := []VideoCodec{CodecH264, CodecHEVC, CodecAV1, CodecVP9}
	for _, codec := range codecs {
		if caps.PiProfile != nil && codec == CodecH264 && caps.PiProfile.DefaultH264Encoder != "" {
			if support, ok := caps.CodecMatrix[codec]; ok && support.Preferred.Accel != AccelNone {
				hardwareSelected = true
			}
			continue // Pi profile already pinned mandatory H.264 path
		}
		sel, attempts := selectEncodeForDeviceChain(caps, codec, devices, hierarchy)
		caps.DeviceAttempts = append(caps.DeviceAttempts, attempts...)
		if sel.Encoder == "" {
			if !caps.AllowSoftware {
				if support, ok := caps.CodecMatrix[codec]; ok {
					support.Preferred = EncoderSelection{}
					caps.CodecMatrix[codec] = support
				}
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
				if sel.Accel != AccelNone {
					hardwareSelected = true
				}
			}
		}
	}
	if !caps.AllowSoftware && !hardwareSelected {
		return errNoHardwareEncoder
	}
	return nil
}

func selectEncodeForDeviceChain(caps *Capabilities, codec VideoCodec, devices []platform.GPUChoice, globalHierarchy []AccelType) (EncoderSelection, []DeviceCodecAttempt) {
	var attempts []DeviceCodecAttempt
	for _, dev := range devices {
		if dev.SoftwareOnly {
			att := DeviceCodecAttempt{Codec: codec, Device: deviceLabel(dev)}
			for _, sw := range SoftwareFallback(codec) {
				if caps.EncoderAvailable(sw) {
					att.Encoder = sw
					att.Selected = true
					attempts = append(attempts, att)
					return EncoderSelection{Encoder: sw, Accel: AccelNone, Kind: encoderKind(sw)}, attempts
				}
			}
			att.Reason = "no software encoder available"
			attempts = append(attempts, att)
			continue
		}
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

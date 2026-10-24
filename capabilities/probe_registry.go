package capabilities

import (
	"context"
	"sort"
	"strings"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// EncoderProbeResults returns structured probe rows for hardware encoders (stable order).
func (c *Capabilities) EncoderProbeResults() []EncoderProbeResult {
	if c == nil {
		return nil
	}
	if len(c.EncoderProbes) > 0 {
		out := append([]EncoderProbeResult(nil), c.EncoderProbes...)
		sort.Slice(out, func(i, j int) bool { return out[i].Encoder < out[j].Encoder })
		return out
	}
	var out []EncoderProbeResult
	for _, known := range KnownEncoders {
		if !known.HW {
			continue
		}
		enc, ok := c.Encoders[known.Name]
		if !ok {
			continue
		}
		out = append(out, EncoderProbeResult{
			Encoder:   known.Name,
			Accel:     accelForEncoderKind(known.Kind),
			Available: enc.Available,
			Code:      enc.ProbeCode,
			Hint:      enc.ProbeHint,
			Detail:    enc.ProbeDetail,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Encoder < out[j].Encoder })
	return out
}

// HardwareProbeSummaryLine is a compact status string for admin UIs (e.g. FileBrowser).
func (c *Capabilities) HardwareProbeSummaryLine() string {
	if c == nil {
		return "hardware probes: unavailable"
	}
	link := c.FFmpegBinary.Kind
	if link == "" {
		link = FFmpegLinkUnknown
	}
	available := c.HardwareEncoders()
	if len(available) == 0 {
		return "hw probes: ffmpeg=" + string(link) + " | hardware encoders: none"
	}
	return "hw probes: ffmpeg=" + string(link) + " | available: " + strings.Join(available, ", ")
}

// MarkEncoderUnhealthy flags an encoder as failed at runtime and clears availability.
func (c *Capabilities) MarkEncoderUnhealthy(encoderName, stderr string) EncoderProbeResult {
	if c == nil {
		return EncoderProbeResult{Encoder: encoderName}
	}
	kind := encoderKind(encoderName)
	code, hint := ClassifyHWProbeFailure(stderr, nil, kind)
	if code == HWProbeOK {
		code = HWProbeUnknown
		hint = HintForProbeCode(code)
	}
	probe := EncoderProbeResult{
		Encoder:   encoderName,
		Accel:     accelForEncoderKind(kind),
		Available: false,
		Code:      code,
		Hint:      hint,
		Detail:    truncateProbeDetail(stderr, 400),
	}
	if enc, ok := c.Encoders[encoderName]; ok {
		ApplyEncoderProbe(&enc, probe)
		c.Encoders[encoderName] = enc
	}
	c.refreshEncoderProbeIndex(probe)
	return probe
}

func (c *Capabilities) refreshEncoderProbeIndex(probe EncoderProbeResult) {
	for i, p := range c.EncoderProbes {
		if p.Encoder == probe.Encoder {
			c.EncoderProbes[i] = probe
			return
		}
	}
	c.EncoderProbes = append(c.EncoderProbes, probe)
}

// ReProbeHardwareEncoders re-runs layered detection for the given encoders (or all HW encoders when empty).
func ReProbeHardwareEncoders(ctx context.Context, caps *Capabilities, runner *ffexec.Runner, plat platform.Info, hierarchy []AccelType, gpuScoped bool, encoderNames ...string) error {
	if caps == nil || runner == nil {
		return nil
	}
	targets := encoderNames
	if len(targets) == 0 {
		for _, known := range KnownEncoders {
			if known.HW {
				targets = append(targets, known.Name)
			}
		}
	}
	gpus, _ := platform.ListGPUDevices()
	binary := caps.FFmpegBinary
	if binary.Kind == "" {
		binary = ProbeFFmpegBinary(runner.FFmpegPath)
		caps.FFmpegBinary = binary
	}
	for _, name := range targets {
		knownKind := encoderKind(name)
		enc := caps.Encoders[name]
		if !enc.Compiled && !encoderActuallyCompiled(ctx, runner, name, false) {
			continue
		}
		if reason := platformSkipReason(knownKind, plat); reason != "" {
			probe := EncoderProbeResult{Encoder: name, Accel: accelForEncoderKind(knownKind), Code: HWProbeSkipped, Hint: reason}
			ApplyEncoderProbe(&enc, probe)
			enc.TestError = reason
			caps.Encoders[name] = enc
			caps.refreshEncoderProbeIndex(probe)
			continue
		}
		smokePlat := PlatformForEncoderKind(plat, knownKind, gpus)
		probe := ProbeHardwareEncoder(ctx, runner, LayeredProbeInput{
			EncoderName: name,
			EncoderKind: knownKind,
			Platform:    plat,
			Binary:      binary,
		}, smokePlat)
		enc.Tested = true
		ApplyEncoderProbe(&enc, probe)
		outOfSelection := gpuScoped && !encoderKindInHierarchy(knownKind, hierarchy)
		enc.OutOfSelection = outOfSelection
		if outOfSelection {
			if probe.Available {
				enc.SelectionNote = "available, not selected"
			} else {
				enc.SelectionNote = "skipped: outside selected device"
			}
		}
		caps.Encoders[name] = enc
		caps.refreshEncoderProbeIndex(probe)
	}
	BuildCodecMatrix(caps, hierarchy)
	return nil
}

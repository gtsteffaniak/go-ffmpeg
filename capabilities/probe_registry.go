package capabilities

import (
	"sort"
	"strings"
)

// EncoderProbeResults returns structured probe rows for hardware encoders (stable order).
// Populated once during startup detection; the library does not update these at runtime.
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

func (c *Capabilities) refreshEncoderProbeIndex(probe EncoderProbeResult) {
	for i, p := range c.EncoderProbes {
		if p.Encoder == probe.Encoder {
			c.EncoderProbes[i] = probe
			return
		}
	}
	c.EncoderProbes = append(c.EncoderProbes, probe)
}

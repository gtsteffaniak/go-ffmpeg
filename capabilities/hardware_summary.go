package capabilities

import (
	"sort"
	"strings"
)

// HardwareSupportSummary returns one short line describing hardware codec
// support and whether it should be logged as a warning.
//
//	hardware encode: VAAPI [h264 hevc], QSV [h264 hevc] | decode: h264 hevc av1 vp9 | no hw encode: av1 vp9 (GPU limit)
//
// warn is true only when support is incomplete for a reason a driver/package
// install can resolve (for example the NVIDIA userspace libraries are missing);
// the line then ends with "| fix: ...". GPU capability limits (AV1 encode on
// Gen12/Ampere) never set warn. Safe to call after detection; read-only.
func (c *Capabilities) HardwareSupportSummary() (line string, warn bool) {
	if c == nil {
		return "", false
	}
	codecs := []VideoCodec{CodecH264, CodecHEVC, CodecAV1, CodecVP9}

	// Hardware encode, grouped per backend in KnownEncoders order.
	var encodeParts []string
	groups := map[string][]string{}
	var order []string
	for _, known := range KnownEncoders {
		if !known.HW {
			continue
		}
		enc, ok := c.Encoders[known.Name]
		if !ok || !enc.Available {
			continue
		}
		label := vendorBackendLabel(accelForEncoderKind(known.Kind))
		if _, seen := groups[label]; !seen {
			order = append(order, label)
		}
		groups[label] = appendUnique(groups[label], string(encoderCodecFamily(known.Name)))
	}
	// Preferred backend first, following the resolved encoder hierarchy.
	rank := map[string]int{}
	for i, accel := range c.EncoderHierarchy {
		rank[vendorBackendLabel(accel)] = i + 1
	}
	sort.SliceStable(order, func(i, j int) bool {
		ri, rj := rank[order[i]], rank[order[j]]
		if ri == 0 {
			ri = len(rank) + 1
		}
		if rj == 0 {
			rj = len(rank) + 1
		}
		return ri < rj
	})
	for _, label := range order {
		encodeParts = append(encodeParts, label+" ["+strings.Join(groups[label], " ")+"]")
	}

	// Hardware decode.
	var decode []string
	for _, codec := range codecs {
		if sel := c.CodecMatrix[codec].DecodePreferred; sel.Decoder != "" && sel.Accel != AccelNone && sel.Accel != "" {
			decode = append(decode, string(codec))
		}
	}

	// Capability limits and driver-fixable gaps from the probes.
	var limited []string
	driverBackends := map[string]bool{}
	var driverOrder, fixes []string
	for _, probe := range c.EncoderProbeResults() {
		if probe.Available {
			continue
		}
		switch {
		case probe.Code == HWProbeCodecUnsupported || probe.Code == HWProbeNVENCUnsupported:
			limited = appendUnique(limited, string(encoderCodecFamily(probe.Encoder)))
		case probe.Code == HWProbeGPUPresentDriverMissing:
			label := vendorBackendLabel(probe.Accel)
			if !driverBackends[label] {
				driverBackends[label] = true
				driverOrder = append(driverOrder, label)
			}
			fixes = appendUnique(fixes, shortFix(probe.Hint))
		}
	}
	sort.Strings(limited)
	// A codec only counts as "limited" if no backend encodes it in hardware.
	limited = removeEncoded(limited, groups)

	var parts []string
	switch {
	case len(encodeParts) > 0:
		parts = append(parts, "hardware encode: "+strings.Join(encodeParts, ", "))
	case len(driverOrder) == 0:
		parts = append(parts, "hardware encode: none (software only)")
	}
	if len(decode) > 0 {
		parts = append(parts, "decode: "+strings.Join(decode, " "))
	}
	if len(limited) > 0 {
		parts = append(parts, "no hw encode: "+strings.Join(limited, " ")+" (GPU limit)")
	}
	if len(driverOrder) > 0 {
		parts = append(parts, "missing driver: "+strings.Join(driverOrder, ", "))
		if len(fixes) > 0 {
			parts = append(parts, "fix: "+strings.Join(fixes, "; "))
		}
		warn = true
	}
	return strings.Join(parts, " | "), warn
}

// shortFix extracts the actionable part of a remediation hint, dropping the
// leading "GPU detected (...) but ... is unavailable on this host" sentence.
func shortFix(hint string) string {
	segs := strings.Split(hint, "; ")
	if len(segs) > 1 && strings.HasPrefix(segs[0], "GPU detected") {
		segs = segs[1:]
	}
	return strings.Join(segs, "; ")
}

func appendUnique(list []string, v string) []string {
	if v == "" {
		return list
	}
	for _, e := range list {
		if e == v {
			return list
		}
	}
	return append(list, v)
}

func removeEncoded(limited []string, encoded map[string][]string) []string {
	have := map[string]bool{}
	for _, codecs := range encoded {
		for _, c := range codecs {
			have[c] = true
		}
	}
	var out []string
	for _, c := range limited {
		if !have[c] {
			out = append(out, c)
		}
	}
	return out
}

package capabilities

import "strings"

// startupWarnKind distinguishes merged warn lines so filtered legacy views can
// separate gpu_present_driver_missing rows from capability-limit and Vulkan rows.
type startupWarnKind int

const (
	warnKindGPUPresent startupWarnKind = iota
	warnKindCapability
	warnKindVulkan
)

type startupWarnLine struct {
	kind startupWarnKind
	text string
}

// StartupHardwareWarnLines returns warn-level startup lines for hardware probes
// that failed for a host-actionable reason. It is the single de-duplicated
// source for [Service.Reload] logging: vendor-backend and Vulkan failures are
// merged and reported once per distinct remediation instead of once per encoder.
//
// Lines are ordered by encoder name for stable output. Safe to call after
// detection; read-only.
func (c *Capabilities) StartupHardwareWarnLines() []string {
	lines := c.startupWarnLines()
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.text)
	}
	return out
}

func (c *Capabilities) startupWarnLines() []startupWarnLine {
	if c == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []startupWarnLine
	var vulkanEncoders []string
	var vulkanHint string
	var vulkanCode HWProbeCode

	add := func(kind startupWarnKind, line string) {
		if line == "" {
			return
		}
		key := strings.ToLower(strings.TrimSpace(line))
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, startupWarnLine{kind: kind, text: line})
	}

	for _, probe := range c.EncoderProbeResults() {
		if probe.Available {
			continue
		}
		if probe.Accel == AccelVulkan {
			// Vulkan failures share one remediation; collect the encoder names
			// so the operator gets a single line instead of one per codec.
			if !c.VulkanEncoderRequested || !vulkanProbeWarnCode(probe.Code) {
				continue
			}
			hint := probe.Hint
			if extra := vulkanVendorRemediation(c.DetectionHost, probe.Code); extra != "" && !strings.Contains(strings.ToLower(hint), strings.ToLower(extra)) {
				hint = strings.TrimSpace(hint + " | " + extra)
			}
			if vulkanHint == "" {
				vulkanHint, vulkanCode = hint, probe.Code
			} else if !strings.EqualFold(hint, vulkanHint) || probe.Code != vulkanCode {
				// Distinct message (for example a different failure class).
				add(warnKindVulkan, vulkanWarnLine([]string{probe.Encoder}, probe.Code, hint))
				continue
			}
			vulkanEncoders = append(vulkanEncoders, probe.Encoder)
			continue
		}
		switch probe.Code {
		case HWProbeGPUPresentDriverMissing:
			add(warnKindGPUPresent, "hardware probe: "+probe.Encoder+": "+probe.Hint)
		}
		// Capability limits (NVENC unsupported, hw_codec_unsupported) are not
		// warnings; see [Capabilities.StartupHardwareInfoLines].
	}
	add(warnKindVulkan, vulkanWarnLine(vulkanEncoders, vulkanCode, vulkanHint))

	if len(out) == 0 {
		for vendor, msg := range c.vendorLevelRemediation() {
			add(warnKindGPUPresent, "hardware probe: "+vendor+": "+msg)
		}
	}
	return out
}

// StartupHardwareInfoLines returns info-level startup lines for hardware
// capability limits: encoders whose backend works but whose GPU cannot encode
// the codec (for example AV1 on Intel Gen12 or NVIDIA Ampere). These are
// expected on many GPUs, carry no install advice, and are reported as one line.
func (c *Capabilities) StartupHardwareInfoLines() []string {
	if c == nil {
		return nil
	}
	var names []string
	for _, probe := range c.EncoderProbeResults() {
		if probe.Available {
			continue
		}
		if probe.Code == HWProbeCodecUnsupported || probe.Code == HWProbeNVENCUnsupported {
			names = append(names, probe.Encoder)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return []string{"hardware encode not supported by this GPU (hardware limit, not a driver problem): " +
		strings.Join(names, ", ") + "; other hardware codecs or software encoding are used for these"}
}

// vulkanWarnLine formats one Vulkan failure line, naming every affected encoder.
func vulkanWarnLine(encoders []string, code HWProbeCode, hint string) string {
	if len(encoders) == 0 || hint == "" {
		return ""
	}
	line := "vulkan probe: " + strings.Join(encoders, ", ")
	if code != "" {
		line += ": [" + string(code) + "]"
	}
	return line + " " + hint
}

// vendorBackendLabel names the backend family for a capability-limit line
// without "Vulkan/NVENC" style duplication already present in the encoder name.
func vendorBackendLabel(accel AccelType) string {
	switch accel {
	case AccelNVENC:
		return "NVENC"
	case AccelAMF:
		return "AMF"
	case AccelQSV:
		return "QSV"
	case AccelVAAPI, AccelD3D12:
		return "VAAPI"
	case AccelVulkan:
		return "Vulkan"
	case AccelV4L2M2M:
		return "V4L2 M2M"
	default:
		return "hardware"
	}
}

// warnLinesOfKind returns merged warn lines of the requested kind.
func (c *Capabilities) warnLinesOfKind(kinds ...startupWarnKind) []string {
	var out []string
	for _, l := range c.startupWarnLines() {
		for _, k := range kinds {
			if l.kind == k {
				out = append(out, l.text)
				break
			}
		}
	}
	return out
}

package capabilities

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

const (
	reportBackendColWidth  = 26
	reportEncoderColWidth  = 20
	reportCompiledColWidth = 13 // "compiled=yes"
	reportRoleColWidth     = 11 // "encode: yes"
	reportColGap           = 2
)

// reportColStyled pads styled text using plain-text width so ANSI codes do not skew columns.
func reportColStyled(plain, styled string, width int) string {
	pad := width - len(plain)
	if pad < 0 {
		pad = 0
	}
	return styled + strings.Repeat(" ", pad) + strings.Repeat(" ", reportColGap)
}

// ReportString returns a human-readable capability report.
func (c *Capabilities) ReportString() string {
	return c.ReportStringWithOptions(ReportOptions{})
}

// ReportStringWithOptions returns a formatted report with optional color.
func (c *Capabilities) ReportStringWithOptions(opts ReportOptions) string {
	var b strings.Builder
	_ = c.ReportWithOptions(&b, opts)
	return b.String()
}

// Report writes a human-readable capability report to w.
func (c *Capabilities) Report(w io.Writer) error {
	return c.ReportWithOptions(w, ReportOptions{})
}

// ReportWithOptions writes a formatted report with optional ANSI colors.
func (c *Capabilities) ReportWithOptions(w io.Writer, opts ReportOptions) error {
	if c == nil {
		return fmt.Errorf("capabilities is nil")
	}
	st := newReportStyle(reportColorEnabled(w, opts))

	fmt.Fprintln(w, st.bold(st.magenta("=== go-ffmpeg capability report ===")))
	fmt.Fprintf(w, "%s ffmpeg %s @ %s\n",
		st.bold("Binary:"), st.cyan(c.FFmpegVersion), st.dim(c.FFmpegPath))
	fmt.Fprintf(w, "%s %s @ %s\n",
		st.bold("FFprobe:"), st.cyan(c.FFprobeVersion), st.dim(c.FFprobePath))

	writeBuildSection(w, st, c)
	writeSystemPlatformSummary(w, st, c)
	writeSelectedGPUSection(w, st, c)
	writeHardwareBackendSections(w, st, c)
	writeCodecResolutionSection(w, st, c)

	if len(c.EnabledOps) > 0 {
		fmt.Fprintf(w, "%s\n", st.section("Operations enabled:"))
		for i, op := range c.EnabledOps {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprint(w, st.green(op))
		}
		fmt.Fprintln(w)
	}
	if len(c.DisabledOps) > 0 {
		fmt.Fprintln(w, st.section("Operations disabled:"))
		for name, reasons := range c.DisabledOps {
			fmt.Fprintf(w, "  %s: %s\n", st.red(name), st.dim(strings.Join(reasons, "; ")))
		}
	}
	if len(c.DeprecatedOps) > 0 {
		fmt.Fprintf(w, "%s %s\n", st.section("Operations deprecated:"),
			st.dim(strings.Join(c.DeprecatedOps, ", ")+" (available, not advertised)"))
	}
	return nil
}

// HardwareEncoders returns the hardware encoders that passed detection, sorted
// for stable output. It ignores the encoder preference hierarchy, so detected
// hardware is still reported when software encoding is preferred.
func (c *Capabilities) HardwareEncoders() []string {
	if c == nil {
		return nil
	}
	var names []string
	for name, enc := range c.Encoders {
		if enc.Available && isHardwareKind(enc.Kind) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// HardwareDecoders returns the hardware decoders that passed detection, sorted
// for stable output.
func (c *Capabilities) HardwareDecoders() []string {
	if c == nil {
		return nil
	}
	var names []string
	for name, dec := range c.Decoders {
		if dec.Available && isHardwareKind(dec.Kind) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func isHardwareKind(kind string) bool {
	return kind != "" && kind != "software" && kind != "native"
}

// SummaryLine returns a single-line capability summary for startup logging.
func (c *Capabilities) SummaryLine() string {
	if c == nil {
		return "capability detection unavailable"
	}

	version := c.FFmpegVersion
	if version == "" {
		version = "unknown"
	}
	profile := c.BuildProfile
	if profile == "" {
		profile = BuildUnknown
	}
	parts := []string{fmt.Sprintf("ffmpeg %s (%s)", version, profile)}

	hierarchy := c.EncoderHierarchy
	if len(hierarchy) == 0 {
		hierarchy = HierarchyForPlatform(c.Platform)
	}
	if labels := accelLabelsFor(hierarchy); len(labels) > 0 {
		parts = append(parts, "encoder preference: "+strings.Join(labels, " → "))
	}

	if hw := c.HardwareEncoders(); len(hw) > 0 {
		parts = append(parts, "hardware encoders: "+strings.Join(hw, ", "))
	} else {
		parts = append(parts, "hardware encoders: none")
	}
	if c.FFmpegBinary.Kind != "" {
		parts = append(parts, "ffmpeg link: "+string(c.FFmpegBinary.Kind))
	}

	ops := fmt.Sprintf("operations: %d enabled", len(c.EnabledOps))
	if len(c.DisabledOps) > 0 {
		ops += fmt.Sprintf(", %d disabled", len(c.DisabledOps))
	}
	parts = append(parts, ops)

	return strings.Join(parts, " | ")
}

func accelLabelsFor(hierarchy []AccelType) []string {
	labels := make([]string, 0, len(hierarchy))
	for _, accel := range hierarchy {
		labels = append(labels, AccelLabel(accel))
	}
	return labels
}

func writeHints(w io.Writer, st reportStyle, msg string) {
	for _, hint := range strings.Split(msg, "\n") {
		hint = strings.TrimSpace(hint)
		if hint == "" {
			continue
		}
		fmt.Fprintf(w, "      %s\n", st.dim("↳ "+hint))
	}
}

func isVP9IntelLinuxEncodeUnavailable(encoderName string, plat platform.Info) bool {
	if plat.OS != "linux" || !plat.Intel {
		return false
	}
	return encoderName == "vp9_qsv" || encoderName == "vp9_vaapi"
}

// JSON returns the capability matrix as JSON.
func (c *Capabilities) JSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

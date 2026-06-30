package capabilities

import (
	"encoding/json"
	"fmt"
	"io"
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
	return nil
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

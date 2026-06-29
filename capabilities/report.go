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
	fmt.Fprintf(w, "%s %s\n", st.bold("Build profile:"), st.profileLabel(c.BuildProfile))

	writePlatformSection(w, st, c)
	writeHWAccelSection(w, st, c)

	fmt.Fprintln(w, st.section("Video codecs (encode / decode):"))
	writeCodecGroups(w, st, c)

	fmt.Fprintln(w, st.section("Codec resolution:"))
	for _, codec := range []VideoCodec{CodecH264, CodecAV1, CodecVP9, CodecHEVC} {
		support, ok := c.CodecMatrix[codec]
		if !ok {
			continue
		}
		codecName := CodecLabel(codec)
		fmt.Fprintf(w, "  %s\n", st.bold(codecName))
		p := support.Preferred
		if p.Encoder != "" {
			fmt.Fprintf(w, "    %s %s%s",
				st.dim("encode ->"),
				st.encoderLabel(p.Encoder, p.Accel),
				st.resolutionAccelSuffix(p.Accel),
			)
			if p.Fallback != "" && p.Fallback != p.Encoder {
				fmt.Fprintf(w, " | %s %s", st.dim("fallback"), st.fallbackLabel(p.Fallback, p.Accel))
			}
			fmt.Fprintln(w)
		} else {
			fmt.Fprintf(w, "    %s %s\n", st.dim("encode ->"), st.red("none"))
		}
		d := support.DecodePreferred
		if d.Decoder != "" {
			fmt.Fprintf(w, "    %s %s%s",
				st.dim("decode ->"),
				st.decoderLabel(d.Decoder, d.Accel),
				st.resolutionAccelSuffix(d.Accel),
			)
			if d.Fallback != "" && d.Fallback != d.Decoder {
				fmt.Fprintf(w, " | %s %s", st.dim("fallback"), st.fallbackDecoderLabel(d.Fallback, d.Accel))
			}
			fmt.Fprintln(w)
		} else {
			fmt.Fprintf(w, "    %s %s\n", st.dim("decode ->"), st.red("none"))
		}
	}

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

func writeCodecGroups(w io.Writer, st reportStyle, c *Capabilities) {
	for _, group := range encoderReportGroups {
		fmt.Fprintf(w, "  %s\n", st.bold(group.Title))
		vp9IntelEncodeHintShown := false
		for _, name := range group.Encoders {
			enc, ok := c.Encoders[name]
			if !ok {
				continue
			}
			backendLabel := BackendDisplayLabel(enc.Name, enc.Kind)
			encodeAvail, decodeAvail, _, decodeErr := c.EncodeDecodeSummary(name)
			_, hasDecode := DecodeBindingForEncoder(name)

			encoderPlain := "(" + enc.Name + ")"
			line := "    " +
				reportColStyled(backendLabel, st.kindLabel(enc.Kind, backendLabel), reportBackendColWidth) +
				reportColStyled(encoderPlain, st.dim(encoderPlain), reportEncoderColWidth) +
				reportColStyled(st.compiledFieldPlain(enc.Compiled), st.compiledField(enc.Compiled), reportCompiledColWidth) +
				reportColStyled(st.roleStatusPlain("encode", encodeAvail), st.roleStatus("encode", encodeAvail), reportRoleColWidth)
			if hasDecode {
				line += reportColStyled(st.roleStatusPlain("decode", decodeAvail), st.roleStatus("decode", decodeAvail), reportRoleColWidth)
			}
			fmt.Fprintln(w, line)

			skipVP9Dup := group.Codec == CodecVP9 && isVP9IntelLinuxEncodeUnavailable(name, c.Platform) && vp9IntelEncodeHintShown
			if enc.TestError != "" && !skipVP9Dup {
				writeHints(w, st, enc.TestError)
				if isVP9IntelLinuxEncodeUnavailable(name, c.Platform) {
					vp9IntelEncodeHintShown = true
				}
			}
			if decodeErr != "" && decodeErr != enc.TestError && !skipVP9Dup {
				writeHints(w, st, decodeErr)
			}
			if hasDecode && !encodeAvail && decodeAvail && !isVP9IntelLinuxEncodeUnavailable(name, c.Platform) {
				fmt.Fprintf(w, "      %s\n", st.dim("↳ decode-only on this GPU/driver (encode unavailable)"))
			}
			if hasDecode && encodeAvail && !decodeAvail {
				fmt.Fprintf(w, "      %s\n", st.dim("↳ encode-only on this GPU/driver (decode unavailable)"))
			}
		}
	}
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

func writeEncoderGroups(w io.Writer, st reportStyle, c *Capabilities) {
	writeCodecGroups(w, st, c)
}

func writePlatformSection(w io.Writer, st reportStyle, c *Capabilities) {
	p := c.Platform
	fmt.Fprintf(w, "%s %s/%s\n", st.bold("Platform:"), st.cyan(p.OS), st.cyan(p.Arch))
	if p.OS == "darwin" {
		fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("VideoToolbox")+":"), st.boolLabel(c.HWAccels["videotoolbox"].Compiled))
	}
	if gpu := p.Details["gpu"]; gpu != "" {
		fmt.Fprintf(w, "  %s %s\n", st.bold("GPU:"), st.dim(gpu))
	}
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("DRI")+":"), st.boolLabel(p.DRI))
	if dev := p.Details["render_device"]; dev != "" {
		fmt.Fprintf(w, "      %s %s\n", st.dim("render node"), st.cyan(dev))
	}
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("Intel")+":"), st.boolLabel(p.Intel))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("QSV")+":"), st.boolLabel(p.QSV))
	if p.Intel {
		fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("VPL")+":"), st.vplLabel(p.Details["vpl_dispatcher"]))
		fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("QSVRuntime")+":"), st.qsvRuntimeLabel(p))
		if lib := p.Details["qsv_runtime_lib"]; lib != "" {
			fmt.Fprintf(w, "      %s %s\n", st.dim("runtime"), st.cyan(lib))
		} else if hint := p.Details["qsv_runtime_hint"]; hint != "" {
			fmt.Fprintf(w, "      %s %s\n", st.bold("hint:"), st.dim(hint))
		}
	}
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("VAAPI")+":"), st.boolLabel(p.VAAPI))
	if drv := p.Details["vaapi_driver"]; drv != "" {
		fmt.Fprintf(w, "      %s %s\n", st.dim("driver"), st.cyan(drv))
	} else if hint := p.Details["intel_va_hint"]; hint != "" {
		fmt.Fprintf(w, "      %s %s\n", st.bold("hint:"), st.dim(hint))
	}
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("NVIDIA")+":"), st.boolLabel(p.NVIDIA))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("AMD")+":"), st.boolLabel(p.AMD))
	if p.Intel && !p.AMD && !p.NVIDIA {
		fmt.Fprintln(w, st.dim("  Hardware profile: Intel-only — VAAPI needs intel-media-va-driver-non-free; QSV also needs libmfx-gen1.2 (oneVPL GPU runtime)"))
	}
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("D3D12")+":"), st.boolLabel(p.D3D12))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("WSL")+":"), st.boolLabel(p.WSL))
	fmt.Fprintln(w, st.dim("  Note: platform gates detect drivers/GPU presence; rows below show runtime FFmpeg encode/decode smoke-test results."))
}

func writeHWAccelSection(w io.Writer, st reportStyle, c *Capabilities) {
	if len(c.HWAccels) == 0 {
		return
	}
	fmt.Fprintln(w, st.section("FFmpeg hwaccels (compiled):"))
	names := make([]string, 0, len(c.HWAccels))
	for name := range c.HWAccels {
		names = append(names, name)
	}
	// stable-ish order
	for _, name := range []string{"cuda", "dxva2", "d3d11va", "d3d12va", "qsv", "vaapi", "vulkan", "videotoolbox", "vdpau"} {
		if c.HWAccels[name].Compiled {
			fmt.Fprintf(w, "  %s\n", st.cyan(name))
		}
	}
	for _, name := range names {
		if _, listed := map[string]bool{"cuda": true, "dxva2": true, "d3d11va": true, "d3d12va": true, "qsv": true, "vaapi": true, "vulkan": true, "videotoolbox": true, "vdpau": true}[name]; listed {
			continue
		}
		fmt.Fprintf(w, "  %s\n", st.cyan(name))
	}
}

// JSON returns the capability matrix as JSON.
func (c *Capabilities) JSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

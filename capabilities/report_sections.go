package capabilities

import (
	"fmt"
	"io"
	"strings"
)

type hwReportBackend struct {
	Title string
	Kinds []string
	Accel AccelType
}

var hwReportBackends = []hwReportBackend{
	{Title: "Software (CPU)", Kinds: []string{"software", "native"}, Accel: AccelNone},
	{Title: "NVIDIA NVENC / NVDEC", Kinds: []string{"nvenc"}, Accel: AccelNVENC},
	{Title: "Intel Quick Sync (QSV)", Kinds: []string{"qsv"}, Accel: AccelQSV},
	{Title: "VAAPI", Kinds: []string{"vaapi"}, Accel: AccelVAAPI},
	{Title: "AMD AMF", Kinds: []string{"amf"}, Accel: AccelAMF},
	{Title: "Apple VideoToolbox", Kinds: []string{"videotoolbox"}, Accel: AccelVideoToolbox},
}

func encodersForKinds(kinds []string) []string {
	kindSet := make(map[string]struct{}, len(kinds))
	for _, k := range kinds {
		kindSet[k] = struct{}{}
	}
	var names []string
	for _, known := range KnownEncoders {
		if _, ok := kindSet[known.Kind]; ok {
			names = append(names, known.Name)
		}
	}
	return names
}

func backendVisible(c *Capabilities, backend hwReportBackend) bool {
	if c == nil || c.SelectedGPU == nil || !c.SelectedGPU.Enabled {
		return true
	}
	if backend.Accel == AccelNone {
		return true
	}
	return hierarchyIncludesAccel(c.EncoderHierarchy, backend.Accel)
}

func writeBuildSection(w io.Writer, st reportStyle, c *Capabilities) {
	fmt.Fprintln(w, st.section("FFmpeg build (compiled with):"))
	fmt.Fprintf(w, "  %s %s\n", st.bold("Profile:"), st.profileLabel(c.BuildProfile))
	if len(c.BuildConfig.LibFlags) > 0 {
		fmt.Fprintf(w, "  %s %s\n", st.bold("Libraries:"), st.cyan(strings.Join(trimLibFlags(c.BuildConfig.LibFlags), ", ")))
	}
	if len(c.HWAccels) > 0 {
		fmt.Fprintf(w, "  %s %s\n", st.bold("HW accels:"), st.cyan(strings.Join(compiledHWAccels(c), ", ")))
	}
	if filters := compiledFilterNames(c); len(filters) > 0 {
		fmt.Fprintf(w, "  %s %s\n", st.bold("Key filters:"), st.dim(strings.Join(filters, ", ")))
	}
	if protocols := compiledProtocolNames(c); len(protocols) > 0 {
		fmt.Fprintf(w, "  %s %s\n", st.bold("Protocols:"), st.dim(strings.Join(protocols, ", ")))
	}
	if c.BuildConfig.RawLine != "" {
		line := c.BuildConfig.RawLine
		if len(line) > 120 {
			line = line[:117] + "..."
		}
		fmt.Fprintf(w, "  %s %s\n", st.bold("Configure:"), st.dim(line))
	}
}

func trimLibFlags(flags []string) []string {
	out := make([]string, 0, len(flags))
	for _, f := range flags {
		out = append(out, strings.TrimPrefix(f, "--enable-"))
	}
	return out
}

func compiledHWAccels(c *Capabilities) []string {
	order := []string{"cuda", "dxva2", "d3d11va", "d3d12va", "qsv", "vaapi", "vulkan", "videotoolbox", "vdpau"}
	var out []string
	for _, name := range order {
		if c.HWAccels[name].Compiled {
			out = append(out, name)
		}
	}
	for name, hw := range c.HWAccels {
		if !hw.Compiled {
			continue
		}
		seen := false
		for _, o := range out {
			if o == name {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, name)
		}
	}
	return out
}

func compiledFilterNames(c *Capabilities) []string {
	var out []string
	for _, name := range KnownFilters {
		if c.Filters[name] {
			out = append(out, name)
		}
	}
	return out
}

func compiledProtocolNames(c *Capabilities) []string {
	var out []string
	for _, name := range KnownProtocols {
		if c.Protocols[name] {
			out = append(out, name)
		}
	}
	return out
}

func writeSelectedGPUSection(w io.Writer, st reportStyle, c *Capabilities) {
	if c.SelectedGPU == nil || !c.SelectedGPU.Enabled {
		return
	}
	gpu := c.SelectedGPU
	fmt.Fprintln(w, st.section("Selected GPU:"))
	fmt.Fprintf(w, "  %s %s\n", st.bold("Device:"), st.cyan(gpu.Name))
	if gpu.Vendor != "" {
		fmt.Fprintf(w, "  %s %s\n", st.bold("Vendor:"), st.cyan(gpu.Vendor))
	}
	if dev := c.Platform.Details["render_device"]; dev != "" {
		fmt.Fprintf(w, "  %s %s\n", st.bold("Render node:"), st.cyan(dev))
	}
	if len(c.EncoderHierarchy) > 0 {
		labels := make([]string, 0, len(c.EncoderHierarchy))
		for _, accel := range c.EncoderHierarchy {
			labels = append(labels, AccelLabel(accel))
		}
		fmt.Fprintf(w, "  %s %s\n", st.bold("Encoder hierarchy:"), st.dim(strings.Join(labels, " → ")))
	}
	writeScopedPlatformGates(w, st, c)
}

func writeScopedPlatformGates(w io.Writer, st reportStyle, c *Capabilities) {
	p := c.Platform
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("Intel")+":"), st.boolLabel(p.Intel))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("QSV")+":"), st.boolLabel(p.QSV))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("QSVRuntime")+":"), st.qsvRuntimeLabel(p))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("VAAPI")+":"), st.boolLabel(p.VAAPI))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("NVIDIA")+":"), st.boolLabel(p.NVIDIA))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("AMD")+":"), st.boolLabel(p.AMD))
}

func writeHardwareBackendSections(w io.Writer, st reportStyle, c *Capabilities) {
	for _, backend := range hwReportBackends {
		if !backendVisible(c, backend) {
			continue
		}
		names := encodersForKinds(backend.Kinds)
		if len(names) == 0 {
			continue
		}
		if !backendHasCompiledEncoders(c, names) && backend.Accel != AccelNone {
			continue
		}
		fmt.Fprintln(w, st.section(backend.Title+":"))
		writeEncoderRows(w, st, c, names)
	}
}

func backendHasCompiledEncoders(c *Capabilities, names []string) bool {
	for _, name := range names {
		if enc, ok := c.Encoders[name]; ok && enc.Compiled {
			return true
		}
	}
	return false
}

func writeEncoderRows(w io.Writer, st reportStyle, c *Capabilities, encoderNames []string) {
	byCodec := map[VideoCodec][]string{}
	for _, name := range encoderNames {
		enc, ok := c.Encoders[name]
		if !ok || (!enc.Compiled && enc.TestError == "") {
			continue
		}
		codec := encoderCodecFamily(name)
		byCodec[codec] = append(byCodec[codec], name)
	}
	for _, group := range encoderReportGroups {
		if group.Codec == "" {
			continue
		}
		names, ok := byCodec[group.Codec]
		if !ok {
			continue
		}
		fmt.Fprintf(w, "  %s\n", st.bold(group.Title))
		vp9IntelEncodeHintShown := false
		for _, name := range names {
			writeSingleEncoderRow(w, st, c, group, name, &vp9IntelEncodeHintShown)
		}
	}
	// misc encoders (mjpeg, aac, etc.)
	for _, name := range encoderNames {
		enc, ok := c.Encoders[name]
		if !ok {
			continue
		}
		if encoderCodecFamily(name) != "" {
			continue
		}
		writeSingleEncoderRow(w, st, c, encoderReportGroup{Title: "Other"}, name, nil)
		if enc.TestError != "" {
			writeHints(w, st, enc.TestError)
		}
	}
}

func writeSingleEncoderRow(w io.Writer, st reportStyle, c *Capabilities, group encoderReportGroup, name string, vp9IntelEncodeHintShown *bool) {
	enc, ok := c.Encoders[name]
	if !ok {
		return
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

	skipVP9Dup := group.Codec == CodecVP9 && vp9IntelEncodeHintShown != nil &&
		isVP9IntelLinuxEncodeUnavailable(name, c.Platform) && *vp9IntelEncodeHintShown
	if enc.TestError != "" && !skipVP9Dup {
		writeHints(w, st, enc.TestError)
		if vp9IntelEncodeHintShown != nil && isVP9IntelLinuxEncodeUnavailable(name, c.Platform) {
			*vp9IntelEncodeHintShown = true
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

func encoderCodecFamily(name string) VideoCodec {
	switch {
	case strings.HasPrefix(name, "h264") || name == "libx264":
		return CodecH264
	case strings.HasPrefix(name, "hevc") || name == "libx265" || name == "libvvenc":
		return CodecHEVC
	case strings.HasPrefix(name, "av1") || strings.HasPrefix(name, "libaom") || strings.HasPrefix(name, "libsvtav1") || name == "librav1e":
		return CodecAV1
	case strings.HasPrefix(name, "vp9") || name == "libvpx-vp9":
		return CodecVP9
	default:
		return ""
	}
}

func writeCodecResolutionSection(w io.Writer, st reportStyle, c *Capabilities) {
	fmt.Fprintln(w, st.section("Codec resolution:"))
	if c.SelectedGPU != nil && c.SelectedGPU.Enabled {
		fmt.Fprintln(w, st.dim("  Paths below reflect the selected GPU and its encoder hierarchy."))
	} else if len(c.EncoderHierarchy) > 0 {
		labels := make([]string, 0, len(c.EncoderHierarchy))
		for _, accel := range c.EncoderHierarchy {
			labels = append(labels, AccelLabel(accel))
		}
		fmt.Fprintf(w, "  %s %s\n", st.dim("Hierarchy:"), st.dim(strings.Join(labels, " → ")))
	}
	for _, codec := range []VideoCodec{CodecH264, CodecAV1, CodecVP9, CodecHEVC} {
		support, ok := c.CodecMatrix[codec]
		if !ok {
			continue
		}
		fmt.Fprintf(w, "  %s\n", st.bold(CodecLabel(codec)))
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
}

func writeSystemPlatformSummary(w io.Writer, st reportStyle, c *Capabilities) {
	if c.SelectedGPU != nil && c.SelectedGPU.Enabled {
		return
	}
	p := c.Platform
	fmt.Fprintln(w, st.section("System platform:"))
	fmt.Fprintf(w, "  %s %s/%s\n", st.bold("OS:"), st.cyan(p.OS), st.cyan(p.Arch))
	if gpu := p.Details["gpu"]; gpu != "" {
		fmt.Fprintf(w, "  %s %s\n", st.bold("GPUs detected:"), st.dim(gpu))
	}
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("NVIDIA")+":"), st.boolLabel(p.NVIDIA))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("Intel")+":"), st.boolLabel(p.Intel))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("AMD")+":"), st.boolLabel(p.AMD))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("VAAPI")+":"), st.boolLabel(p.VAAPI))
	fmt.Fprintf(w, "  %s %s\n", st.bold(PlatformGateLabel("QSV")+":"), st.boolLabel(p.QSV))
	fmt.Fprintln(w, st.dim("  Runtime rows below show smoke-test results for every hardware backend detected on this system."))
}

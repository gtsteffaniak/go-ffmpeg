package capabilities

import (
	"io"
	"os"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// ANSI color codes for terminal output.
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiWhite   = "\033[37m"
)

// ReportOptions configures report formatting.
type ReportOptions struct {
	// Color enables ANSI colors. When nil, color is auto-detected from w.
	Color *bool
}

type reportStyle struct {
	on bool
}

func newReportStyle(enabled bool) reportStyle {
	return reportStyle{on: enabled}
}

func (s reportStyle) wrap(code, text string) string {
	if !s.on || text == "" {
		return text
	}
	return code + text + ansiReset
}

func (s reportStyle) bold(text string) string    { return s.wrap(ansiBold, text) }
func (s reportStyle) dim(text string) string     { return s.wrap(ansiDim, text) }
func (s reportStyle) red(text string) string     { return s.wrap(ansiRed, text) }
func (s reportStyle) green(text string) string   { return s.wrap(ansiGreen, text) }
func (s reportStyle) yellow(text string) string  { return s.wrap(ansiYellow, text) }
func (s reportStyle) blue(text string) string    { return s.wrap(ansiBlue, text) }
func (s reportStyle) magenta(text string) string { return s.wrap(ansiMagenta, text) }
func (s reportStyle) cyan(text string) string    { return s.wrap(ansiCyan, text) }

func (s reportStyle) boolLabel(v bool) string {
	if v {
		return s.green("yes")
	}
	return s.dim("no")
}

func (s reportStyle) availLabel(v bool) string {
	if v {
		return s.green("available")
	}
	return s.red("unavailable")
}

func (s reportStyle) roleStatus(role string, avail bool) string {
	return s.dim(role+":") + " " + s.availLabel(avail)
}

func (s reportStyle) vplLabel(path string) string {
	if path != "" {
		return s.green("yes")
	}
	return s.red("no")
}

func (s reportStyle) qsvRuntimeLabel(p platform.Info) string {
	if p.QSVRuntime {
		return s.green("yes")
	}
	if p.QSV && p.VAAPI {
		return s.yellow("missing")
	}
	return s.dim("no")
}

func (s reportStyle) compiledLabel(v bool) string {
	if v {
		return s.green("true")
	}
	return s.red("false")
}

func (s reportStyle) section(title string) string {
	return s.bold("--- " + title)
}

func (s reportStyle) kindLabel(kind, display string) string {
	if display == "" {
		display = kind
	}
	switch kind {
	case "nvenc":
		return s.green(display)
	case "amf":
		return s.yellow(display)
	case "qsv":
		return s.cyan(display)
	case "vaapi":
		return s.magenta(display)
	case "software", "native":
		return s.dim(display)
	default:
		return display
	}
}

func (s reportStyle) encoderLabel(name string, accel AccelType) string {
	label := EncoderLabel(name)
	if isSoftwareEncoderPath(name, accel) {
		return s.dim(label)
	}
	return s.kindLabel(encoderKind(name), label)
}

func (s reportStyle) decoderLabel(name string, accel AccelType) string {
	label := DecoderLabel(name)
	if isSoftwareDecoderPath(name, accel) {
		return s.dim(label)
	}
	return s.kindLabel(decoderKindForName(name), label)
}

func isSoftwareEncoderPath(name string, accel AccelType) bool {
	if accel == AccelNone {
		return true
	}
	switch encoderKind(name) {
	case "software", "native":
		return true
	default:
		return false
	}
}

func isSoftwareDecoderPath(name string, accel AccelType) bool {
	if accel == AccelNone {
		return true
	}
	if strings.HasPrefix(name, "hwaccel:") {
		return false
	}
	return decoderKindForName(name) == "software"
}

func (s reportStyle) accelLabel(accel AccelType) string {
	label := AccelLabel(accel)
	if accel == AccelNone {
		return s.dim(label)
	}
	switch accel {
	case AccelNVENC:
		return s.green(label)
	case AccelAMF:
		return s.yellow(label)
	case AccelQSV:
		return s.cyan(label)
	case AccelVAAPI, AccelD3D12:
		return s.magenta(label)
	default:
		return label
	}
}

func (s reportStyle) fallbackLabel(encoder string, _ AccelType) string {
	return s.dim(EncoderLabel(encoder))
}

func (s reportStyle) fallbackDecoderLabel(decoder string, _ AccelType) string {
	return s.dim(DecoderLabel(decoder))
}

func (s reportStyle) resolutionAccelSuffix(accel AccelType) string {
	if accel == AccelNone {
		return ""
	}
	return " (" + s.accelLabel(accel) + ")"
}

func (s reportStyle) white(text string) string { return s.wrap(ansiWhite, text) }

func (s reportStyle) profileLabel(p BuildProfile) string {
	switch p {
	case BuildFull:
		return s.green(string(p))
	case BuildDecodeOnly:
		return s.yellow(string(p))
	default:
		return s.yellow(string(p))
	}
}

// IsTerminalWriter reports whether w is an interactive character device.
func IsTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func reportColorEnabled(w io.Writer, opts ReportOptions) bool {
	if opts.Color != nil {
		return *opts.Color
	}
	return IsTerminalWriter(w)
}

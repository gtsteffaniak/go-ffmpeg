package capabilities

import (
	"regexp"
	"strings"
)

var versionRe = regexp.MustCompile(`ffmpeg version (\S+)`)
var ffprobeVersionRe = regexp.MustCompile(`ffprobe version (\S+)`)
var libFlagRe = regexp.MustCompile(`--enable-[^\s']+`)

// ParseVersionOutput extracts version and build config from ffmpeg -version output.
func ParseVersionOutput(output string) (version string, cfg BuildConfig) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if m := versionRe.FindStringSubmatch(line); len(m) == 2 {
			version = m[1]
		}
		if strings.HasPrefix(line, "configuration:") {
			cfg.RawLine = line
			flags := libFlagRe.FindAllString(line, -1)
			cfg.Flags = flags
			for _, f := range flags {
				if strings.HasPrefix(f, "--enable-lib") {
					cfg.LibFlags = append(cfg.LibFlags, f)
				}
			}
		}
	}
	return version, cfg
}

// ParseFFprobeVersion extracts version from ffprobe -version output.
func ParseFFprobeVersion(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if m := ffprobeVersionRe.FindStringSubmatch(strings.TrimSpace(line)); len(m) == 2 {
			return m[1]
		}
	}
	return ""
}

// InferBuildProfile infers full vs decode-only from lib flags and encoder list.
func InferBuildProfile(cfg BuildConfig, encoders map[string]EncoderCapability) BuildProfile {
	encodeMarkers := []string{"libx264", "libsvtav1", "libvpx-vp9", "libx265"}
	found := 0
	for _, name := range encodeMarkers {
		if enc, ok := encoders[name]; ok && enc.Compiled {
			found++
		}
	}
	if found >= 2 {
		return BuildFull
	}
	for _, f := range cfg.LibFlags {
		if f == "--enable-libx264" || f == "--enable-libsvtav1" {
			return BuildFull
		}
	}
	if found == 0 {
		return BuildDecodeOnly
	}
	return BuildUnknown
}

// ParseListOutput parses ffmpeg -encoders/-decoders/-filters/-protocols style output.
func ParseListOutput(output string) []string {
	var names []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Lines look like: "V..... libx264 description..."
		name := fields[1]
		if name == "=" || strings.Contains(name, ".") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// ParseHWAccelsOutput parses ffmpeg -hwaccels output (one method name per line).
func ParseHWAccelsOutput(output string) []string {
	var names []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 1 {
			names = append(names, fields[0])
			continue
		}
		if len(fields) >= 2 {
			names = append(names, fields[1])
		}
	}
	return names
}

// ParseEncoderHelp checks if encoder help output confirms an encoder exists.
func ParseEncoderHelp(output, encoderName string) bool {
	return strings.Contains(output, encoderName)
}

// ParseDecoderHelp checks if decoder help output confirms a decoder exists.
func ParseDecoderHelp(output, decoderName string) bool {
	return strings.Contains(output, decoderName)
}

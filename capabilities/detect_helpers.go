package capabilities

import (
	"context"
	"strings"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// ProbeEncoder checks if an encoder is compiled in via -h encoder=.
func ProbeEncoder(ctx context.Context, runner *ffexec.Runner, name string) (bool, string) {
	res, err := runner.RunFFmpeg(ctx, "-hide_banner", "-h", "encoder="+name)
	if err != nil {
		return false, res.Stderr
	}
	if ParseEncoderHelp(res.Stdout+res.Stderr, name) {
		return true, ""
	}
	return false, "encoder not listed in help output"
}

// PopulateEncoders fills encoder capabilities in the matrix.
func PopulateEncoders(ctx context.Context, caps *Capabilities, runner *ffexec.Runner, skipHW bool, plat platform.Info) {
	listRes, err := runner.RunFFmpeg(ctx, "-hide_banner", "-encoders")
	compiledSet := map[string]bool{}
	if err == nil {
		for _, name := range ParseListOutput(listRes.Stdout) {
			compiledSet[name] = true
		}
	}

	for _, known := range KnownEncoders {
		enc := EncoderCapability{Name: known.Name, Kind: known.Kind}
		enc.Compiled = encoderActuallyCompiled(ctx, runner, known.Name, compiledSet[known.Name])
		if !enc.Compiled {
			caps.Encoders[known.Name] = enc
			continue
		}

		if known.HW {
			if skipHW {
				enc.TestError = "hardware tests skipped (-skip-hw-tests)"
				caps.Encoders[known.Name] = enc
				continue
			}
			if reason := platformSkipReason(known.Kind, plat); reason != "" {
				enc.TestError = reason
				caps.Encoders[known.Name] = enc
				continue
			}
			if !platformGateForEncoder(known.Kind, plat) {
				enc.TestError = platformSkipReason(known.Kind, plat)
				caps.Encoders[known.Name] = enc
				continue
			}
			enc.Tested = true
			ok, testErr := SmokeTestHardwareEncoder(ctx, runner, known.Name, plat)
			enc.Available = ok
			enc.TestError = testErr
		} else {
			enc.Available = true
		}
		caps.Encoders[known.Name] = enc
	}
}

// PopulateDecoders fills decoder capabilities including hardware decode smoke tests.
func PopulateDecoders(ctx context.Context, caps *Capabilities, runner *ffexec.Runner, skipHW bool, plat platform.Info) {
	listRes, err := runner.RunFFmpeg(ctx, "-hide_banner", "-decoders")
	compiledSet := map[string]bool{}
	if err == nil {
		for _, name := range ParseListOutput(listRes.Stdout) {
			compiledSet[name] = true
		}
	}

	fixtures := newCodecFixtures()
	defer fixtures.cleanup()

	for _, known := range KnownDecoders {
		dec := DecoderCapability{
			Name:    known.Name,
			Kind:    known.Kind,
			Codec:   known.Codec,
			HWAccel: known.HWAccel,
		}
		if known.HWAccel != "" && strings.HasPrefix(known.Name, "hwaccel:vaapi:") {
			dec.SWCodec = strings.TrimPrefix(known.Name, "hwaccel:vaapi:")
		}

		dec.Compiled = decoderActuallyCompiled(ctx, runner, known.Name, compiledSet[known.Name], known.HWAccel, dec.SWCodec, caps)
		if !dec.Compiled {
			caps.Decoders[known.Name] = dec
			continue
		}

		if known.HW {
			if skipHW {
				dec.TestError = "hardware tests skipped (-skip-hw-tests)"
				caps.Decoders[known.Name] = dec
				continue
			}
			if reason := platformSkipReason(known.Kind, plat); reason != "" {
				dec.TestError = reason
				caps.Decoders[known.Name] = dec
				continue
			}
			if !platformGateForEncoder(known.Kind, plat) {
				dec.TestError = platformSkipReason(known.Kind, plat)
				caps.Decoders[known.Name] = dec
				continue
			}
			dec.Tested = true
			ok, testErr := SmokeTestHardwareDecoder(ctx, runner, known.Name, known.Codec, known.HWAccel, dec.SWCodec, plat, fixtures)
			dec.Available = ok
			dec.TestError = testErr
		} else {
			dec.Available = true
		}
		caps.Decoders[known.Name] = dec
	}
}

// PopulateFilters fills filter availability.
func PopulateFilters(ctx context.Context, caps *Capabilities, runner *ffexec.Runner) {
	res, err := runner.RunFFmpeg(ctx, "-hide_banner", "-filters")
	listed := map[string]bool{}
	if err == nil {
		for _, name := range ParseListOutput(res.Stdout) {
			listed[name] = true
		}
	}
	for _, name := range KnownFilters {
		if listed[name] {
			caps.Filters[name] = true
			continue
		}
		helpRes, helpErr := runner.RunFFmpeg(ctx, "-hide_banner", "-h", "filter="+name)
		caps.Filters[name] = helpErr == nil && strings.Contains(helpRes.Stdout, name)
	}
}

// PopulateHWAccels fills hwaccel list.
func PopulateHWAccels(ctx context.Context, caps *Capabilities, runner *ffexec.Runner) {
	res, err := runner.RunFFmpeg(ctx, "-hide_banner", "-hwaccels")
	if err != nil {
		return
	}
	output := res.Stdout + res.Stderr
	for _, name := range ParseHWAccelsOutput(output) {
		caps.HWAccels[name] = HWAccelCapability{Name: name, Compiled: true, Available: true}
	}
}

// PopulateProtocols fills protocol list.
func PopulateProtocols(ctx context.Context, caps *Capabilities, runner *ffexec.Runner) {
	res, err := runner.RunFFmpeg(ctx, "-hide_banner", "-protocols")
	listed := map[string]bool{}
	if err == nil {
		for _, name := range ParseProtocolsOutput(res.Stdout) {
			listed[name] = true
		}
	}
	for _, name := range KnownProtocols {
		caps.Protocols[name] = listed[name]
	}
}

// ParseProtocolsOutput parses ffmpeg -protocols output.
func ParseProtocolsOutput(output string) []string {
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

// BuildCodecMatrix resolves preferred encoders and decoders per codec family.
func BuildCodecMatrix(caps *Capabilities, hierarchy []AccelType) {
	codecs := []VideoCodec{CodecH264, CodecAV1, CodecVP9, CodecHEVC}
	for _, codec := range codecs {
		support := CodecSupport{
			Hardware:       make(map[AccelType]string),
			HardwareDecode: make(map[AccelType]string),
		}
		for _, sw := range SoftwareFallback(codec) {
			if caps.EncoderAvailable(sw) {
				support.Software = append(support.Software, sw)
			}
		}
		for _, sw := range SoftwareDecodeFallback(codec) {
			if caps.DecoderAvailable(sw) {
				support.SoftwareDecode = append(support.SoftwareDecode, sw)
			}
		}
		for _, accel := range hierarchy {
			encName := CodecEncoderMap(codec, accel)
			if encName != "" && caps.EncoderAvailable(encName) {
				support.Hardware[accel] = encName
			}
			if decName := CodecDecoderMap(codec, accel); decName != "" && caps.DecoderAvailable(decName) {
				support.HardwareDecode[accel] = decName
			} else if key := CodecHWAccelDecodeKey(codec, accel); key != "" && caps.DecoderAvailable(key) {
				support.HardwareDecode[accel] = key
			}
		}
		support.Preferred = resolvePreferredEncode(support, hierarchy)
		support.DecodePreferred = resolvePreferredDecode(caps, support, hierarchy)
		caps.CodecMatrix[codec] = support
	}
	BuildEncodeOptions(caps)
	BuildDecodeOptions(caps)
}

func resolvePreferredEncode(support CodecSupport, hierarchy []AccelType) EncoderSelection {
	for _, accel := range hierarchy {
		if enc, ok := support.Hardware[accel]; ok {
			kind := encoderKind(enc)
			sel := EncoderSelection{Encoder: enc, Accel: accel, Kind: kind}
			if len(support.Software) > 0 {
				sel.Fallback = support.Software[0]
			}
			return sel
		}
	}
	if len(support.Software) > 0 {
		return EncoderSelection{Encoder: support.Software[0], Accel: AccelNone, Kind: "software", Fallback: support.Software[0]}
	}
	return EncoderSelection{Accel: AccelNone}
}

func resolvePreferredDecode(caps *Capabilities, support CodecSupport, hierarchy []AccelType) DecoderSelection {
	for _, accel := range hierarchy {
		if dec, ok := support.HardwareDecode[accel]; ok {
			sel := DecoderSelection{Decoder: dec, Accel: accel, Kind: decoderKindForName(dec)}
			if decCap, ok := caps.Decoders[dec]; ok {
				sel.HWAccel = decCap.HWAccel
				sel.SWCodec = decCap.SWCodec
			}
			if len(support.SoftwareDecode) > 0 {
				sel.Fallback = support.SoftwareDecode[0]
			}
			return sel
		}
	}
	if len(support.SoftwareDecode) > 0 {
		return DecoderSelection{
			Decoder:  support.SoftwareDecode[0],
			Accel:    AccelNone,
			Kind:     "software",
			Fallback: support.SoftwareDecode[0],
		}
	}
	return DecoderSelection{Accel: AccelNone}
}

func encoderKind(name string) string {
	for _, e := range KnownEncoders {
		if e.Name == name {
			return e.Kind
		}
	}
	return "unknown"
}

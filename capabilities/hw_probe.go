package capabilities

import (
	"context"
	"strings"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

const hwSmokeDurationSec = "0.5"

// LayeredProbeInput carries binary and host context for one hardware encoder probe.
type LayeredProbeInput struct {
	EncoderName string
	EncoderKind string
	Platform    platform.Info
	Binary      FFmpegBinaryInfo
	SkipSmoke   bool
}

// ProbeHardwareEncoder runs binary → prerequisite → functional layers for one encoder.
func ProbeHardwareEncoder(ctx context.Context, runner *ffexec.Runner, in LayeredProbeInput, smokePlat platform.Info) EncoderProbeResult {
	res := EncoderProbeResult{
		Encoder: in.EncoderName,
		Accel:   accelForEncoderKind(in.EncoderKind),
	}

	if gpuAccelBlockedByStaticBinary(in.Binary, in.EncoderKind) {
		res.Code = HWProbeStaticBinary
		res.Hint = HintForProbeCode(res.Code)
		res.Detail = "ffmpeg binary is statically linked on Linux"
		return res
	}

	if code, hint := CheckHostPrerequisites(in.EncoderKind, in.Platform); code != HWProbeOK {
		res.Code = code
		res.Hint = hint
		if res.Hint == "" {
			res.Hint = HintForProbeCode(code)
		}
		return res
	}

	if in.SkipSmoke {
		res.Code = HWProbeSkipped
		res.Hint = HintForProbeCode(HWProbeSkipped)
		return res
	}

	ok, detail := functionalHWProbe(ctx, runner, in.EncoderName, smokePlat)
	if ok {
		res.Available = true
		return res
	}
	res.Code, res.Hint = ClassifyHWProbeFailure(detail, nil, in.EncoderKind)
	if res.Hint == "" {
		res.Hint = HintForProbeCode(res.Code)
	}
	res.Detail = truncateProbeDetail(detail, 400)
	return res
}

func functionalHWProbe(ctx context.Context, runner *ffexec.Runner, encoderName string, plat platform.Info) (bool, string) {
	testCtx, cancel := context.WithTimeout(ctx, hwTestTimeout)
	defer cancel()

	args := hwSmokeArgs(encoderName, plat)
	if args == nil {
		return false, "unsupported hardware encoder for smoke test"
	}
	// Normalize smoke duration to the layered probe standard.
	for i := 0; i < len(args); i++ {
		if args[i] == "-t" && i+1 < len(args) {
			args[i+1] = hwSmokeDurationSec
		}
		if strings.HasPrefix(args[i], "testsrc=duration=") {
			args[i] = strings.Replace(args[i], "duration=0.1", "duration="+hwSmokeDurationSec, 1)
		}
	}

	res, err := runner.RunFFmpegProcessGroup(testCtx, args...)
	output := res.Stdout + res.Stderr
	if err != nil {
		if testCtx.Err() == context.DeadlineExceeded {
			return false, output + "\nprobe timed out after " + time.Duration(hwTestTimeout).String()
		}
		kind := encoderKindForName(encoderName)
		primary, hints := hwTroubleshoot(encoderName, kind, output, plat)
		if primary == "" {
			primary = summarizeHWError(output)
		}
		return false, formatTestMessages(primary, hints) + "\n" + output
	}
	for _, pattern := range hwFailurePatterns {
		if strings.Contains(output, pattern) {
			kind := encoderKindForName(encoderName)
			primary, hints := hwTroubleshoot(encoderName, kind, output, plat)
			if primary == "" {
				primary = summarizeHWError(output)
			}
			return false, formatTestMessages(primary, hints) + "\n" + output
		}
	}
	return true, ""
}

// ApplyEncoderProbe copies a probe result into EncoderCapability and returns the test error string.
func ApplyEncoderProbe(enc *EncoderCapability, probe EncoderProbeResult) {
	if enc == nil {
		return
	}
	enc.Available = probe.Available
	enc.ProbeCode = probe.Code
	enc.ProbeHint = probe.Hint
	enc.ProbeDetail = probe.Detail
	if probe.Available {
		enc.TestError = ""
		return
	}
	enc.TestError = formatProbeError(probe.Code, probe.Hint, probe.Detail)
}

package capabilities

import (
	"context"
	"strings"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

const hwSmokeDurationSec = "0.5"

// LayeredProbeInput carries context for a single [ProbeHardwareEncoder] call.
type LayeredProbeInput struct {
	EncoderName string
	EncoderKind string
	Platform    platform.Info
	Binary      FFmpegBinaryInfo
	Host        DetectionHost
	// SkipSmoke skips the functional encode when HW tests are disabled.
	SkipSmoke bool
}

// ProbeHardwareEncoder runs binary, prerequisite, and functional layers for one encoder.
// It does not retry or mutate global state; the caller stores the returned
// [EncoderProbeResult] on [Capabilities]. ctx bounds the functional probe only.
func ProbeHardwareEncoder(ctx context.Context, runner *ffexec.Runner, in LayeredProbeInput, smokePlat platform.Info) EncoderProbeResult {
	res := EncoderProbeResult{
		Encoder: in.EncoderName,
		Accel:   accelForEncoderKind(in.EncoderKind),
	}

	if gpuAccelBlockedByStaticBinary(in.Binary, in.EncoderKind) {
		res.Code = HWProbeStaticBinary
		res.Hint = HintForProbeCode(res.Code)
		res.Detail = "ffmpeg binary is statically linked on Linux"
		res.OriginalCode = res.Code
		res.Code, res.Hint = ApplyHostRemediation(in.Host, in.EncoderKind, in.Binary, res.Code, res.Hint)
		return res
	}

	if code, hint := CheckHostPrerequisites(in.EncoderKind, in.Platform); code != HWProbeOK {
		res.Code = code
		res.OriginalCode = code
		res.Hint = hint
		if res.Hint == "" {
			res.Hint = HintForProbeCode(code)
		}
		res.Code, res.Hint = ApplyHostRemediation(in.Host, in.EncoderKind, in.Binary, res.Code, res.Hint)
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
	res.OriginalCode = res.Code
	if res.Hint == "" {
		res.Hint = HintForProbeCode(res.Code)
	}
	res.Detail = compactProbeDetail(detail, 240)
	res.Code, res.Hint = ApplyHostRemediation(in.Host, in.EncoderKind, in.Binary, res.Code, res.Hint)
	return res
}

// functionalHWProbe runs a short testsrc encode in a process group with timeout kill.
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

// ApplyEncoderProbe copies probe into enc and sets TestError from Code, Hint, and Detail.
func ApplyEncoderProbe(enc *EncoderCapability, probe EncoderProbeResult) {
	if enc == nil {
		return
	}
	enc.Available = probe.Available
	enc.ProbeCode = probe.Code
	enc.ProbeOriginal = probe.OriginalCode
	enc.ProbeHint = probe.Hint
	enc.ProbeDetail = probe.Detail
	if probe.Available {
		enc.TestError = ""
		return
	}
	enc.TestError = formatProbeError(probe.Code, probe.Hint, probe.Detail)
}

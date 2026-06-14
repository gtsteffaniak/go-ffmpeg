package capabilities

import (
	"context"
	"strings"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

const hwTestTimeout = 10 * time.Second

var hwFailurePatterns = []string{
	"Error creating a MFX session",
	"No supported child device type",
	"Device creation failed",
	"Unknown encoder",
	"MFX_ERR_UNSUPPORTED",
	"Current codec type is unsupported",
	"Current resolution is unsupported",
	"Invalid FrameType",
	"Error submitting video frame to the encoder",
	"DLL amfrt64.dll failed to open",
	"Failed to initialise VAAPI",
	"Cannot load libva",
}

// SmokeTestHardwareEncoder runs an encode smoke test appropriate for the encoder backend.
func SmokeTestHardwareEncoder(ctx context.Context, runner *ffexec.Runner, encoderName string, plat platform.Info) (bool, string) {
	testCtx, cancel := context.WithTimeout(ctx, hwTestTimeout)
	defer cancel()

	args := hwSmokeArgs(encoderName, plat)
	if args == nil {
		return false, "unsupported hardware encoder for smoke test"
	}

	res, err := runner.RunFFmpeg(testCtx, args...)
	output := res.Stdout + res.Stderr
	if err != nil {
		kind := encoderKindForName(encoderName)
		primary, hints := hwTroubleshoot(encoderName, kind, output, plat)
		if primary == "" {
			primary = summarizeHWError(output)
		}
		return false, formatTestMessages(primary, hints)
	}
	for _, pattern := range hwFailurePatterns {
		if strings.Contains(output, pattern) {
			kind := encoderKindForName(encoderName)
			primary, hints := hwTroubleshoot(encoderName, kind, output, plat)
			if primary == "" {
				primary = summarizeHWError(output)
			}
			return false, formatTestMessages(primary, hints)
		}
	}
	return true, ""
}

func hwSmokeArgs(encoderName string, plat platform.Info) []string {
	renderDev := platform.RenderDevice(plat.Details)

	switch encoderKindForName(encoderName) {
	case "nvenc":
		return []string{
			"-hide_banner", "-f", "lavfi", "-i", "testsrc=duration=0.1:size=320x240:rate=30",
			"-pix_fmt", "yuv420p",
			"-c:v", encoderName, "-t", "0.1", "-f", "null", "-",
		}
	case "amf":
		return []string{
			"-hide_banner", "-f", "lavfi", "-i", "testsrc=duration=0.1:size=320x240:rate=30",
			"-pix_fmt", "yuv420p",
			"-c:v", encoderName, "-t", "0.1", "-f", "null", "-",
		}
	case "qsv":
		return []string{
			"-hide_banner",
			"-init_hw_device", "qsv=hw",
			"-filter_hw_device", "hw",
			"-f", "lavfi", "-i", "testsrc=duration=0.1:size=320x240:rate=30",
			"-vf", "format=nv12,hwupload=extra_hw_frames=64,format=qsv",
			"-c:v", encoderName,
			"-t", "0.1", "-f", "null", "-",
		}
	case "vaapi":
		return []string{
			"-hide_banner",
			"-init_hw_device", "vaapi=va:" + renderDev,
			"-filter_hw_device", "va",
			"-f", "lavfi", "-i", "testsrc=duration=0.1:size=320x240:rate=30",
			"-vf", "format=nv12,hwupload",
			"-c:v", encoderName,
			"-t", "0.1", "-f", "null", "-",
		}
	default:
		return []string{
			"-hide_banner", "-f", "lavfi", "-i", "testsrc=duration=0.1:size=320x240:rate=30",
			"-c:v", encoderName, "-t", "0.1", "-f", "null", "-",
		}
	}
}

func encoderKindForName(name string) string {
	for _, e := range KnownEncoders {
		if e.Name == name {
			return e.Kind
		}
	}
	if strings.Contains(name, "nvenc") {
		return "nvenc"
	}
	if strings.Contains(name, "amf") {
		return "amf"
	}
	if strings.Contains(name, "qsv") {
		return "qsv"
	}
	if strings.Contains(name, "vaapi") {
		return "vaapi"
	}
	return "unknown"
}

func summarizeHWError(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return "hardware encode test failed"
	}

	lines := strings.Split(output, "\n")
	var picks []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "unknown encoder"):
			return "encoder not available in this FFmpeg build"
		case strings.Contains(lower, "error creating a mfx session"):
			return "Intel QSV session failed (MFX -9)"
		case strings.Contains(lower, "cannot load lib"):
			return "required hardware library missing at runtime"
		case strings.Contains(lower, "failed to initialise vaapi"):
			if strings.Contains(lower, "unknown libva error") {
				return "Intel VAAPI driver missing or broken (install intel-media-va-driver-non-free)"
			}
			return "VAAPI initialization failed (driver or render node permissions)"
		case strings.Contains(lower, "impossible to convert between the formats"):
			return "VAAPI format upload failed (fixed in newer detection — retry after update)"
		case strings.Contains(lower, "permission denied") && strings.Contains(lower, "dri"):
			return "permission denied accessing GPU render node (add user to 'render' or 'video' group)"
		case strings.Contains(lower, "encoder not found"):
			return "encoder not available in this FFmpeg build"
		case strings.Contains(lower, "load_plugin"):
			continue // skip noisy qsv option warnings
		case strings.HasPrefix(lower, "[vost") || strings.HasPrefix(lower, "[out#"):
			if strings.Contains(lower, "error") {
				picks = append(picks, trimFFmpegLine(line))
			}
		case strings.HasPrefix(lower, "error"):
			picks = append(picks, trimFFmpegLine(line))
		}
	}
	if len(picks) > 0 {
		msg := picks[len(picks)-1]
		return truncateErr(msg, 160)
	}
	return truncateErr(output, 160)
}

func truncateErr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func trimFFmpegLine(line string) string {
	if idx := strings.Index(line, "]"); idx >= 0 && idx+1 < len(line) {
		return strings.TrimSpace(line[idx+1:])
	}
	return line
}

func platformGateForEncoder(kind string, plat platform.Info) bool {
	switch kind {
	case "nvenc":
		return plat.NVIDIA
	case "amf":
		return plat.AMD
	case "qsv":
		return plat.QSV && plat.Intel && plat.VAAPI && plat.QSVRuntime
	case "vaapi":
		return plat.VAAPI
	default:
		return true
	}
}

func platformSkipReason(kind string, plat platform.Info) string {
	switch kind {
	case "nvenc":
		if !plat.NVIDIA {
			return "[platform] no NVIDIA GPU/driver detected"
		}
	case "amf":
		if !plat.AMD {
			return "[platform] AMF is AMD-only (Intel/NVIDIA GPU detected)"
		}
	case "qsv":
		if !plat.Intel {
			return "[platform] no Intel GPU detected for Quick Sync"
		}
		if !plat.QSV {
			return "[platform] Intel Quick Sync not detected on this host"
		}
		if !plat.VAAPI {
			return "[driver] Intel Quick Sync requires Intel VAAPI driver\nFix: sudo apt install intel-media-va-driver-non-free"
		}
		if !plat.QSVRuntime {
			return qsvRuntimeSkipReason(plat)
		}
	case "vaapi":
		if !plat.VAAPI {
			if plat.Intel {
				return "[driver] Intel VAAPI driver not installed\nFix: sudo apt install intel-media-va-driver-non-free"
			}
			return "[platform] no VAAPI driver found for this GPU"
		}
	}
	return ""
}

func qsvRuntimeSkipReason(plat platform.Info) string {
	msg := "[driver] Intel oneVPL GPU runtime missing (libmfx-gen) — libvpl2 dispatcher alone cannot create QSV sessions"
	if hint := plat.Details["qsv_runtime_hint"]; hint != "" {
		msg += "\nFix: sudo apt install libmfx-gen1.2 intel-media-va-driver-non-free libvpl2 vainfo"
		msg += "\nNote: " + hint
	} else {
		msg += "\nFix: install libmfx-gen / oneVPL GPU runtime for your distro (Ubuntu/Debian: libmfx-gen1.2)"
	}
	if plat.VAAPI {
		msg += "\nWorkaround: VAAPI encode/decode is working — use h264_vaapi/hevc_vaapi/av1_vaapi until QSV runtime is installed"
	}
	return msg
}

// encoderActuallyCompiled verifies the encoder exists beyond a stale list entry.
func encoderActuallyCompiled(ctx context.Context, runner *ffexec.Runner, name string, listed bool) bool {
	res, err := runner.RunFFmpeg(ctx, "-hide_banner", "-h", "encoder="+name)
	if err != nil {
		return false
	}
	body := res.Stdout + res.Stderr
	if !ParseEncoderHelp(body, name) {
		return false
	}
	// Reject if help says unknown or lists no encoder section.
	lower := strings.ToLower(body)
	if strings.Contains(lower, "unknown encoder") {
		return false
	}
	return listed || strings.Contains(body, "Encoder")
}

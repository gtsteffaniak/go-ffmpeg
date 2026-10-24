package capabilities

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

var nvencMinDriverRE = regexp.MustCompile(`Driver version\s+(\d+)\.(\d+)`)

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
	ok, detail := functionalHWProbe(ctx, runner, encoderName, plat)
	if ok {
		return true, ""
	}
	return false, strings.TrimSpace(detail)
}

func hwSmokeArgs(encoderName string, plat platform.Info) []string {
	renderDev := platform.RenderDevice(plat.Details)
	choice := gpuChoiceFromSmokePlatform(plat)

	switch encoderKindForName(encoderName) {
	case "nvenc":
		args := []string{"-hide_banner",
			"-f", "lavfi", "-i", "testsrc=duration=0.5:size=320x240:rate=30",
			"-pix_fmt", "yuv420p",
		}
		args = append(args, platform.FFmpegEncoderDeviceArgs("nvenc", choice, plat)...)
		args = append(args, "-c:v", encoderName, "-t", "0.5", "-f", "null", "-")
		return args
	case "amf":
		args := []string{"-hide_banner"}
		args = append(args, platform.FFmpegHWDeviceInit("amf", choice, plat)...)
		args = append(args,
			"-f", "lavfi", "-i", "testsrc=duration=0.5:size=320x240:rate=30",
			"-pix_fmt", "yuv420p",
			"-c:v", encoderName, "-t", "0.5", "-f", "null", "-",
		)
		return args
	case "v4l2m2m":
		args := []string{
			"-hide_banner", "-f", "lavfi", "-i", "testsrc=duration=0.5:size=320x240:rate=30",
			"-pix_fmt", "yuv420p",
		}
		if strings.Contains(encoderName, "h264") {
			args = append(args, "-b:v", "2M")
		}
		args = append(args, "-c:v", encoderName, "-t", "0.5", "-f", "null", "-")
		return args
	case "qsv":
		args := []string{"-hide_banner"}
		args = append(args, platform.FFmpegHWDeviceInit("qsv", choice, plat)...)
		if len(args) == 1 {
			args = append(args, "-init_hw_device", "qsv=hw", "-filter_hw_device", "hw")
		}
		args = append(args,
			"-f", "lavfi", "-i", "testsrc=duration=0.5:size=320x240:rate=30",
			"-vf", "format=nv12,hwupload=extra_hw_frames=64,format=qsv",
			"-c:v", encoderName,
			"-t", "0.5", "-f", "null", "-",
		)
		return args
	case "vaapi":
		return []string{
			"-hide_banner",
			"-init_hw_device", "vaapi=va:" + renderDev,
			"-filter_hw_device", "va",
			"-f", "lavfi", "-i", "testsrc=duration=0.5:size=320x240:rate=30",
			"-vf", "format=nv12,hwupload",
			"-c:v", encoderName,
			"-t", "0.5", "-f", "null", "-",
		}
	default:
		return []string{
			"-hide_banner", "-f", "lavfi", "-i", "testsrc=duration=0.5:size=320x240:rate=30",
			"-c:v", encoderName, "-t", "0.5", "-f", "null", "-",
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
	if strings.Contains(name, "videotoolbox") {
		return "videotoolbox"
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
		case strings.Contains(lower, "no va display found"):
			return "no VAAPI render device (no /dev/dri node — expected on WSL without GPU-pv DRI)"
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
		case strings.Contains(lower, "driver does not support") || strings.Contains(lower, "minimum required"):
			if msg := nvencDriverMismatch(output); msg != "" {
				return msg
			}
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
	return tailErr(lines, 3, 300)
}

// tailErr keeps the last n non-empty lines. ffmpeg prints its input banner
// first and the failure last, so the head of the output hides the real error.
func tailErr(lines []string, n, max int) string {
	var tail []string
	for i := len(lines) - 1; i >= 0 && len(tail) < n; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			tail = append([]string{line}, tail...)
		}
	}
	msg := strings.Join(tail, " | ")
	if len(msg) > max {
		msg = "..." + msg[len(msg)-max:]
	}
	return msg
}

func truncateErr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func nvencDriverMismatch(output string) string {
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "nvenc") && !strings.Contains(lower, "driver") {
		return ""
	}
	if m := nvencMinDriverRE.FindStringSubmatch(output); len(m) == 3 {
		return fmt.Sprintf("NVENC driver %s.%s below minimum required for this FFmpeg build", m[1], m[2])
	}
	if strings.Contains(lower, "api version") && strings.Contains(lower, "required") {
		return truncateErr(strings.TrimSpace(output), 160)
	}
	return ""
}

func trimFFmpegLine(line string) string {
	if idx := strings.Index(line, "]"); idx >= 0 && idx+1 < len(line) {
		return strings.TrimSpace(line[idx+1:])
	}
	return line
}

func gpuChoiceFromSmokePlatform(plat platform.Info) platform.GPUChoice {
	c := platform.GPUChoice{Enabled: true}
	if plat.Details == nil {
		return c
	}
	if name := plat.Details["smoke_gpu"]; name != "" {
		c.Name = name
	}
	if v := plat.Details["cuda_index"]; v != "" {
		var idx int
		if n, err := fmt.Sscanf(v, "%d", &idx); n == 1 && err == nil {
			c.CUDAIndex = idx
			c.Pinning.NVENC = platform.PinningExact
			c.Pinning.CUDA = platform.PinningExact
		}
	}
	if v := plat.Details["dxgi_index"]; v != "" {
		if n, err := fmt.Sscanf(v, "%d", &c.DXGIIndex); n == 1 && err == nil {
			_ = err
		}
	}
	if dev := plat.Details["render_device"]; dev != "" {
		c.RenderDevice = dev
	}
	return c
}

func platformGateForEncoder(kind string, plat platform.Info) bool {
	switch kind {
	case "nvenc":
		return plat.NVIDIA
	case "amf":
		// AMF is a Windows-only API; an AMD adapter listed on Linux/WSL has
		// no AMF path.
		return plat.AMD && plat.OS == "windows"
	case "v4l2m2m":
		if plat.Details != nil && plat.Details["pi_model"] != "" {
			return len(platform.ListV4L2M2MDevices()) > 0 || plat.Details["pi_model"] == string(platform.Pi4)
		}
		return len(platform.ListV4L2M2MDevices()) > 0
	case "qsv":
		if plat.OS == "windows" {
			// Quick Sync on Windows runs through the Intel graphics driver
			// (D3D11/oneVPL); VAAPI does not exist there. The smoke test decides.
			return plat.Intel
		}
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
			return "[platform] no AMD GPU detected (AMF requires AMD hardware)"
		}
	case "v4l2m2m":
		if !platformGateForEncoder(kind, plat) {
			return "[platform] no V4L2 M2M codec device found (use Raspberry Pi OS ffmpeg on Pi 4+)"
		}
	case "qsv":
		if !plat.Intel {
			return "[platform] no Intel GPU detected for Quick Sync"
		}
		if plat.OS == "windows" {
			return ""
		}
		if !plat.QSV {
			return "[platform] Intel Quick Sync not detected on this host"
		}
		if !plat.VAAPI {
			if plat.WSL && !plat.DRI {
				return "[platform] no VAAPI render device (no /dev/dri node — expected on WSL without GPU-pv DRI)"
			}
			fix := platform.NewPackageHints(plat.Distro).IntelVADriverFix()
			return "[driver] Intel Quick Sync requires Intel VAAPI driver\nFix: " + fix
		}
		if !plat.QSVRuntime {
			return qsvRuntimeSkipReason(plat)
		}
	case "vaapi":
		if !plat.VAAPI {
			if plat.WSL && !plat.DRI {
				return "[platform] no VAAPI render device (no /dev/dri node — expected on WSL without GPU-pv DRI)"
			}
			if plat.Intel {
				fix := platform.NewPackageHints(plat.Distro).IntelVADriverFix()
				return "[driver] Intel VAAPI driver not installed\nFix: " + fix
			}
			return "[platform] no VAAPI driver found for this GPU"
		}
	}
	return ""
}

func qsvRuntimeSkipReason(plat platform.Info) string {
	return platform.NewPackageHints(plat.Distro).QSVRuntimeSkipMessage(plat)
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

package encode

import (
	"context"
	"strings"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

const profileSmokeTimeout = 15 * time.Second

// SmokeTestEncoderArgs runs a minimal lavfi encode using backend-specific input
// filters and the supplied encoder arguments (from VideoEncoderArgs).
func SmokeTestEncoderArgs(ctx context.Context, runner *ffexec.Runner, kind string, encArgs []string, plat platform.Info) (bool, string) {
	testCtx, cancel := context.WithTimeout(ctx, profileSmokeTimeout)
	defer cancel()

	prefix := smokeInputPrefix(kind, plat)
	if prefix == nil {
		return false, "unsupported encoder backend for smoke test"
	}

	args := append(append([]string{}, prefix...), encArgs...)
	args = append(args, "-t", "0.1", "-f", "null", "-")

	res, err := runner.RunFFmpeg(testCtx, args...)
	output := res.Stdout + res.Stderr
	if err != nil {
		return false, summarizeSmokeError(output)
	}
	for _, pattern := range smokeFailurePatterns {
		if strings.Contains(output, pattern) {
			return false, summarizeSmokeError(output)
		}
	}
	return true, ""
}

func smokeInputPrefix(kind string, plat platform.Info) []string {
	renderDev := platform.RenderDevice(plat.Details)
	switch kind {
	case "nvenc", "amf", "software", "videotoolbox", "unknown":
		return []string{
			"-hide_banner", "-f", "lavfi", "-i", "testsrc=duration=0.1:size=320x240:rate=30",
			"-pix_fmt", "yuv420p",
		}
	case "qsv":
		return []string{
			"-hide_banner", "-f", "lavfi", "-i", "testsrc=duration=0.1:size=320x240:rate=30",
		}
	case "vaapi":
		return []string{
			"-hide_banner",
			"-init_hw_device", "vaapi=va:" + renderDev,
			"-filter_hw_device", "va",
			"-f", "lavfi", "-i", "testsrc=duration=0.1:size=320x240:rate=30",
			"-vf", "format=nv12,hwupload",
		}
	default:
		return nil
	}
}

var smokeFailurePatterns = []string{
	"Error creating a MFX session",
	"No supported child device type",
	"Device creation failed",
	"Unknown encoder",
	"MFX_ERR_UNSUPPORTED",
	"Failed to initialise VAAPI",
	"Cannot load libva",
	"Invalid argument",
	"Error while opening encoder",
}

func summarizeSmokeError(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return "encode smoke test failed"
	}
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
			return line
		}
	}
	if len(output) > 200 {
		return output[:200] + "..."
	}
	return output
}

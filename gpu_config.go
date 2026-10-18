package ffmpeg

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// KnownGPUConfigValues lists accepted gpu YAML tokens for help text (FileBrowser and similar).
func KnownGPUConfigValues() []string {
	return []string{
		"auto (default, empty string)",
		"igpu",
		"dgpu",
		"software (aliases: cpu, none)",
		"/dev/dri/renderD* (Linux render node)",
		"GPU name substring (e.g. RTX 3050, Intel)",
	}
}

// GPUConfigHelp returns a short summary for documentation.
func GPUConfigHelp() string {
	return strings.Join([]string{
		`GPUSelector zero / GPUAuto — automatic iGPU→dGPU→software device chain (see DevicePreference)`,
		`GPUIntegrated / GPUDiscrete / GPUSoftware`,
		`GPUByRenderNode(path) / GPUByName(substr)`,
		`ParseGPUSelector(yaml) for string configs; GPUStrictness for error vs fallback`,
	}, "; ")
}

// FormatGPUChoice summarizes a resolved choice for logs or reports.
func FormatGPUChoice(c platform.GPUChoice) string {
	if c.Label != "" {
		base := c.Label
		if c.Fallback && c.FallbackReason != "" {
			return fmt.Sprintf("%s (fallback: %s)", base, c.FallbackReason)
		}
		return base
	}
	if c.SoftwareOnly {
		return "software (CPU only)"
	}
	if c.AutoSelect || c.Mode == platform.GPUModeAuto {
		if c.Fallback {
			return fmt.Sprintf("auto (fallback: %s)", c.FallbackReason)
		}
		return "auto (all GPUs, platform default backend order)"
	}
	parts := []string{string(c.Mode), c.Name}
	if c.RenderDevice != "" {
		parts = append(parts, c.RenderDevice)
	}
	return strings.Join(parts, " · ")
}

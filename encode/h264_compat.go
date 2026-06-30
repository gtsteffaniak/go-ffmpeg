package encode

import (
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

// H264LevelForMaxHeight picks an H.264 level that fits the output height cap.
// Level 3.1 only supports up to 720p; 1080p requires at least 4.0.
func H264LevelForMaxHeight(maxHeight int) string {
	if maxHeight <= 0 {
		maxHeight = 1080
	}
	switch {
	case maxHeight <= 480:
		return "3.0"
	case maxHeight <= 720:
		return "3.1"
	case maxHeight <= 1080:
		return "4.0"
	default:
		return "4.1"
	}
}

// AppendH264FMP4CompatArgs adds browser MSE flags for fragmented MP4 H.264 output.
func AppendH264FMP4CompatArgs(args []string, accel capabilities.AccelType, maxHeight int) []string {
	level := H264LevelForMaxHeight(maxHeight)
	switch accel {
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return append(args, "-tag:v", "avc1")
	default:
		return append(args, "-profile:v", "baseline", "-level", level, "-tag:v", "avc1")
	}
}

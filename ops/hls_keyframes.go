package ops

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

// ProbeVideoKeyframeTimes returns strictly increasing keyframe presentation times in seconds.
// It scans packet flags instead of decoding frames: a frame-level probe
// (-skip_frame nokey) must decode every keyframe, which costs tens of seconds
// of CPU on long files, while packet demuxing is sub-second on the same input.
func ProbeVideoKeyframeTimes(ctx context.Context, runner *ffexec.Runner, path string) ([]float64, error) {
	res, err := runner.RunFFprobe(ctx,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "packet=pts_time,flags",
		"-of", "csv=p=0",
		path,
	)
	if err != nil {
		return nil, err
	}
	var times []float64
	for line := range strings.SplitSeq(strings.TrimSpace(res.Stdout), "\n") {
		line = strings.TrimSpace(line)
		comma := strings.LastIndexByte(line, ',')
		if comma < 0 || !strings.Contains(line[comma+1:], "K") {
			continue
		}
		t, err := strconv.ParseFloat(strings.TrimSpace(line[:comma]), 64)
		if err != nil || t < 0 {
			continue
		}
		if len(times) > 0 && t <= times[len(times)-1]+0.001 {
			continue
		}
		times = append(times, t)
	}
	if len(times) == 0 {
		return nil, fmt.Errorf("no video keyframes found")
	}
	return times, nil
}

package ops

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

// ProbeVideoKeyframeTimes returns strictly increasing keyframe presentation times in seconds.
func ProbeVideoKeyframeTimes(ctx context.Context, runner *ffexec.Runner, path string) ([]float64, error) {
	res, err := runner.RunFFprobe(ctx,
		"-v", "error",
		"-select_streams", "v:0",
		"-skip_frame", "nokey",
		"-show_entries", "frame=best_effort_timestamp_time",
		"-of", "csv=p=0",
		path,
	)
	if err != nil {
		return nil, err
	}
	var times []float64
	for line := range strings.SplitSeq(strings.TrimSpace(res.Stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "N/A") {
			continue
		}
		t, err := strconv.ParseFloat(line, 64)
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

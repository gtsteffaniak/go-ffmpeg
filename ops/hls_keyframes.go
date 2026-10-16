package ops

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

const keyframeTimeDedupeSec = 0.001

// ProbeVideoKeyframeTimes returns strictly increasing keyframe presentation times in seconds.
// It prefers packet demuxing (fast on long files), then falls back to a frame-level probe
// when packet timestamps are unavailable (e.g. raw elementary streams).
func ProbeVideoKeyframeTimes(ctx context.Context, runner *ffexec.Runner, path string) ([]float64, error) {
	times, err := probeVideoKeyframeTimesByPacket(ctx, runner, path)
	if err == nil && len(times) > 0 {
		return times, nil
	}
	frameTimes, frameErr := probeVideoKeyframeTimesByFrame(ctx, runner, path)
	if frameErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, frameErr
	}
	if len(frameTimes) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("no video keyframes found")
	}
	return frameTimes, nil
}

func probeVideoKeyframeTimesByPacket(ctx context.Context, runner *ffexec.Runner, path string) ([]float64, error) {
	res, err := runner.RunFFprobe(ctx,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "packet=pts_time,dts_time,flags",
		"-of", "csv=p=0",
		path,
	)
	if err != nil {
		return nil, err
	}
	var times []float64
	var keyPacketsWithoutTime int
	var totalPackets int
	for line := range strings.SplitSeq(strings.TrimSpace(res.Stdout), "\n") {
		fields := parsePacketKeyframeCSVFields(line)
		if len(fields) < 3 {
			continue
		}
		totalPackets++
		if !strings.Contains(fields[2], "K") {
			continue
		}
		if t, ok := packetKeyframeTimeFromFields(fields); ok {
			times = append(times, t)
			continue
		}
		keyPacketsWithoutTime++
	}
	if len(times) == 0 && keyPacketsWithoutTime > 0 {
		synth, synthErr := syntheticKeyframeTimesFromPackets(ctx, runner, path, totalPackets, keyPacketsWithoutTime)
		if synthErr != nil {
			return nil, synthErr
		}
		return synth, nil
	}
	times = sortDedupeKeyframeTimes(times)
	if len(times) == 0 {
		return nil, fmt.Errorf("no video keyframes found")
	}
	return times, nil
}

func probeVideoKeyframeTimesByFrame(ctx context.Context, runner *ffexec.Runner, path string) ([]float64, error) {
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
		line = strings.TrimSuffix(line, ",")
		if line == "" || strings.EqualFold(line, "N/A") {
			continue
		}
		t, err := strconv.ParseFloat(line, 64)
		if err != nil || t < 0 {
			continue
		}
		times = append(times, t)
	}
	times = sortDedupeKeyframeTimes(times)
	if len(times) == 0 {
		return nil, fmt.Errorf("no video keyframes found")
	}
	return times, nil
}

// parsePacketKeyframeCSVFields splits an ffprobe csv=p=0 line and drops trailing empty fields
// (MPEG-TS lines often end with a dangling comma after the flags column).
func parsePacketKeyframeCSVFields(line string) []string {
	fields := strings.Split(strings.TrimSpace(line), ",")
	for len(fields) > 0 && strings.TrimSpace(fields[len(fields)-1]) == "" {
		fields = fields[:len(fields)-1]
	}
	return fields
}

// packetKeyframeTimeFromCSVLine parses one packet CSV row and returns the keyframe time when
// the flags column marks a key packet. pts_time is preferred; dts_time is used when pts is N/A.
func packetKeyframeTimeFromCSVLine(line string) (float64, bool) {
	fields := parsePacketKeyframeCSVFields(line)
	if len(fields) < 3 || !strings.Contains(fields[2], "K") {
		return 0, false
	}
	return packetKeyframeTimeFromFields(fields)
}

func packetKeyframeTimeFromFields(fields []string) (float64, bool) {
	for _, raw := range []string{fields[0], fields[1]} {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.EqualFold(raw, "N/A") {
			continue
		}
		t, err := strconv.ParseFloat(raw, 64)
		if err != nil || t < 0 {
			continue
		}
		return t, true
	}
	return 0, false
}

func syntheticKeyframeTimesFromPackets(ctx context.Context, runner *ffexec.Runner, path string, totalPackets, keyPackets int) ([]float64, error) {
	if keyPackets <= 0 {
		return nil, fmt.Errorf("no video keyframes found")
	}
	fps, err := ProbeVideoFPS(ctx, runner, path)
	if err != nil || fps <= 0 || elementaryStreamKeyframeFPS(path) > 0 {
		if hint := elementaryStreamKeyframeFPS(path); hint > 0 {
			fps = hint
		} else {
			fps = defaultHLSSegmentFPS
		}
	}
	gopFrames := 1
	if totalPackets > 0 && keyPackets > 0 {
		gopFrames = totalPackets / keyPackets
		if gopFrames < 1 {
			gopFrames = 1
		}
	}
	frameDur := 1.0 / fps
	times := make([]float64, 0, keyPackets)
	for i := range keyPackets {
		times = append(times, float64(i*gopFrames)*frameDur)
	}
	return sortDedupeKeyframeTimes(times), nil
}

func elementaryStreamKeyframeFPS(path string) float64 {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".h264", ".264", ".h265", ".hevc":
		// ffprobe often reports misleading avg_frame_rate on elementary streams; 24fps
		// matches typical HLS source material and keeps GOP spacing stable.
		return 24
	default:
		return 0
	}
}

func sortDedupeKeyframeTimes(times []float64) []float64 {
	if len(times) == 0 {
		return nil
	}
	sort.Float64s(times)
	out := make([]float64, 0, len(times))
	for _, t := range times {
		if len(out) == 0 || t > out[len(out)-1]+keyframeTimeDedupeSec {
			out = append(out, t)
		}
	}
	return out
}

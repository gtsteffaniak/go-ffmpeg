package ops

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

// MotionCrop restricts motion analysis to a rectangle of the frame.
// Values are normalized 0–1 so a zone picked in the UI survives resolution
// changes. Zero value (W or H == 0) means "full frame".
type MotionCrop struct {
	X float64
	Y float64
	W float64
	H float64
}

// Normalized reports whether the crop covers the whole frame (no ROI).
func (c MotionCrop) Normalized() bool {
	return c.W <= 0 || c.H <= 0
}

// normalizedCrop clamps the rectangle into 0–1 and applies a minimum size.
func (c MotionCrop) normalizedCrop() MotionCrop {
	if c.Normalized() {
		return MotionCrop{}
	}
	x := clamp01(c.X)
	y := clamp01(c.Y)
	w := clamp01(c.W)
	h := clamp01(c.H)
	const minSpan = 0.01
	if w < minSpan {
		w = minSpan
	}
	if h < minSpan {
		h = minSpan
	}
	if x+w > 1 {
		w = 1 - x
	}
	if y+h > 1 {
		h = 1 - y
	}
	return MotionCrop{X: x, Y: y, W: w, H: h}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// MotionDetectOptions configures a frame-difference motion analysis pass.
//
// The analysis decodes at SampleFPS, optionally crops to Crop, downscales to
// Width (height derived from the aspect ratio), converts to grayscale, and
// measures the mean absolute difference between consecutive frames
// (tblend=difference + signalstats YAVG). A frame whose score reaches
// Threshold is "motion".
type MotionDetectOptions struct {
	Input InputSource
	RTSP  *RTSPInputOptions

	// SampleFPS is the analysis frame rate (default 2).
	SampleFPS float64
	// Width is the analysis width in pixels (default 320); height follows the aspect ratio.
	Width int
	// Threshold is the mean luma difference (0–255) at or above which a frame counts as motion.
	Threshold float64
	// Crop limits analysis to a normalized rectangle (optional).
	Crop MotionCrop
	// MinDurationSec drops events shorter than this (default 0.5s).
	MinDurationSec float64
	// MergeGapSec merges consecutive motion samples separated by at most this gap.
	// Zero uses 1.5 sample intervals.
	MergeGapSec float64
	// StartSec and EndSec bound the analyzed window; EndSec 0 means end of media.
	StartSec float64
	EndSec   float64

	// DurationSec skips the duration ffprobe when the caller knows the length.
	DurationSec float64
	// Decode selects hardware or software decode.
	Decode  encode.VideoDecodeProfile
	Timeout time.Duration
}

// MotionSample is one analyzed frame's difference score.
type MotionSample struct {
	TimeSec float64
	Score   float64
}

// MotionEvent is a contiguous span scored as motion.
type MotionEvent struct {
	StartSec float64
	EndSec   float64
	// PeakScore is the highest frame-difference score in the event.
	PeakScore float64
	// AvgScore is the mean frame-difference score over the event's samples.
	AvgScore float64
}

// MotionResult is the outcome of a motion analysis pass.
type MotionResult struct {
	Events      []MotionEvent
	Samples     int
	DurationSec float64
	SampleFPS   float64
	Threshold   float64
	PeakScore   float64
	Cropped     bool
	Accel       capabilities.AccelType
	Decoder     string
}

// MotionDefaults holds resolved analysis settings so callers can persist what ran.
type MotionDefaults struct {
	SampleFPS      float64
	Width          int
	Threshold      float64
	MinDurationSec float64
	MergeGapSec    float64
}

// ResolveMotionDefaults fills zero values with the library defaults.
func ResolveMotionDefaults(opts MotionDetectOptions) MotionDefaults {
	fps := opts.SampleFPS
	if fps <= 0 {
		fps = 2
	}
	width := opts.Width
	if width <= 0 {
		width = 320
	}
	threshold := opts.Threshold
	if threshold <= 0 {
		threshold = 2
	}
	minDur := opts.MinDurationSec
	if minDur < 0 {
		minDur = 0
	}
	gap := opts.MergeGapSec
	if gap <= 0 {
		gap = 1.5 / fps
	}
	return MotionDefaults{
		SampleFPS:      fps,
		Width:          width,
		Threshold:      threshold,
		MinDurationSec: minDur,
		MergeGapSec:    gap,
	}
}

// BuildMotionFilter returns the -vf chain used for motion analysis.
func BuildMotionFilter(defaults MotionDefaults, crop MotionCrop) string {
	parts := []string{fmt.Sprintf("fps=%s", formatMotionNumber(defaults.SampleFPS))}
	if c := crop.normalizedCrop(); !c.Normalized() {
		// Crop before scaling so the zone keeps as much detail as possible.
		parts = append(parts, fmt.Sprintf(
			"crop=iw*%s:ih*%s:iw*%s:ih*%s",
			formatMotionNumber(c.W), formatMotionNumber(c.H),
			formatMotionNumber(c.X), formatMotionNumber(c.Y),
		))
	}
	parts = append(parts,
		fmt.Sprintf("scale=%d:-2", defaults.Width),
		"format=gray",
		"tblend=all_mode=difference",
		"signalstats",
		"metadata=print:key=lavfi.signalstats.YAVG:file=-",
	)
	return strings.Join(parts, ",")
}

// formatMotionNumber renders a float without a trailing ".000000".
func formatMotionNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}

// DetectMotion measures frame-to-frame change over a file or stream and returns
// merged motion events. It is a single ffmpeg decode pass; it never encodes.
func DetectMotion(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts MotionDetectOptions) (MotionResult, error) {
	if strings.TrimSpace(opts.Input.URL) == "" {
		return MotionResult{}, fmt.Errorf("motion detect: input URL is required")
	}

	defaults := ResolveMotionDefaults(opts)

	resolver := encode.NewResolver(caps)
	decodeArgs, err := resolver.VideoDecoderArgs(opts.Decode)
	if err != nil {
		// Analysis always has a software fallback: motion detection does not need
		// the requested hardware decoder, so run plain software decode instead.
		decodeArgs = nil
	}
	var sel capabilities.DecoderSelection
	if caps != nil {
		sel, _ = resolver.ResolveDecoder(opts.Decode)
	}

	args := []string{"-hide_banner", "-nostats"}
	args = appendCaptureInputTuning(args, opts.DurationSec > 0, false, nil)
	args = appendInputFlags(args, runner, opts.Input, opts.RTSP, nil)
	args = append(args, decodeArgs...)
	if opts.StartSec > 0 {
		args = append(args, "-ss", formatMotionNumber(opts.StartSec))
	}
	args = append(args, "-i", opts.Input.URL)
	if opts.EndSec > opts.StartSec {
		args = append(args, "-t", formatMotionNumber(opts.EndSec-opts.StartSec))
	}
	args = append(args, "-vf", BuildMotionFilter(defaults, opts.Crop))
	args = appendCaptureOutputTuning(args)
	args = append(args, "-f", "null", "-")

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := runner.RunFFmpeg(pctx, args...)
	if err != nil {
		return MotionResult{}, err
	}

	samples := parseMotionSamples(res.Stdout)
	if len(samples) == 0 {
		samples = parseMotionSamples(res.Stderr)
	}

	result := MotionResult{
		Samples:     len(samples),
		SampleFPS:   defaults.SampleFPS,
		Threshold:   defaults.Threshold,
		DurationSec: opts.DurationSec,
		Cropped:     !opts.Crop.Normalized(),
		Accel:       sel.Accel,
		Decoder:     sel.Decoder,
	}
	for _, s := range samples {
		if s.Score > result.PeakScore {
			result.PeakScore = s.Score
		}
	}
	if result.DurationSec <= 0 && len(samples) > 0 {
		last := samples[len(samples)-1]
		result.DurationSec = last.TimeSec + 1.0/defaults.SampleFPS
	}
	result.Events = mergeMotionEvents(samples, defaults)
	return result, nil
}

// parseMotionSamples reads `pts_time:` / `lavfi.signalstats.YAVG=` pairs from
// ffmpeg's metadata=print output. Metadata lines follow their frame line, but
// parsing them independently keeps this robust across ffmpeg versions.
func parseMotionSamples(out string) []MotionSample {
	if out == "" {
		return nil
	}
	var samples []MotionSample
	var pendingTime float64
	var havePending bool
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "pts_time:"); idx >= 0 {
			rest := line[idx+len("pts_time:"):]
			if end := strings.IndexAny(rest, " \t"); end >= 0 {
				rest = rest[:end]
			}
			if t, err := strconv.ParseFloat(rest, 64); err == nil && !math.IsNaN(t) && t >= 0 {
				pendingTime = t
				havePending = true
			}
			continue
		}
		if idx := strings.Index(line, "lavfi.signalstats.YAVG="); idx >= 0 {
			rest := strings.TrimSpace(line[idx+len("lavfi.signalstats.YAVG="):])
			score, err := strconv.ParseFloat(rest, 64)
			if err != nil || math.IsNaN(score) {
				continue
			}
			if havePending {
				samples = append(samples, MotionSample{TimeSec: pendingTime, Score: score})
				havePending = false
			} else {
				samples = append(samples, MotionSample{Score: score})
			}
		}
	}
	// Fill any missing timestamps from the nominal sample interval.
	if len(samples) > 1 {
		for i := 1; i < len(samples); i++ {
			if samples[i].TimeSec <= samples[i-1].TimeSec {
				samples[i].TimeSec = samples[i-1].TimeSec + 0.5
			}
		}
	}
	return samples
}

// mergeMotionEvents groups consecutive above-threshold samples into events and
// drops events shorter than MinDurationSec.
func mergeMotionEvents(samples []MotionSample, defaults MotionDefaults) []MotionEvent {
	if len(samples) == 0 {
		return nil
	}
	sort.SliceStable(samples, func(i, j int) bool { return samples[i].TimeSec < samples[j].TimeSec })

	interval := 1.0 / defaults.SampleFPS
	if interval <= 0 {
		interval = 0.5
	}

	var events []MotionEvent
	var current *MotionEvent
	var currentSum float64
	var currentCount int

	flush := func() {
		if current == nil {
			return
		}
		// The event ends one interval after the last motion sample.
		start := current.StartSec
		end := current.EndSec + interval
		if end-start >= defaults.MinDurationSec {
			if currentCount > 0 {
				current.AvgScore = currentSum / float64(currentCount)
			}
			current.StartSec = start
			current.EndSec = end
			events = append(events, *current)
		}
		current = nil
		currentSum = 0
		currentCount = 0
	}

	for _, s := range samples {
		if s.Score >= defaults.Threshold {
			if current == nil {
				current = &MotionEvent{StartSec: s.TimeSec, EndSec: s.TimeSec, PeakScore: s.Score}
			} else if s.TimeSec-current.EndSec > defaults.MergeGapSec {
				flush()
				current = &MotionEvent{StartSec: s.TimeSec, EndSec: s.TimeSec, PeakScore: s.Score}
			} else {
				current.EndSec = s.TimeSec
				if s.Score > current.PeakScore {
					current.PeakScore = s.Score
				}
			}
			currentSum += s.Score
			currentCount++
		} else if current != nil && s.TimeSec-current.EndSec > defaults.MergeGapSec {
			flush()
		}
	}
	flush()
	return events
}

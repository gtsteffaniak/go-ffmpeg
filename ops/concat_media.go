package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// Concat actions reported in ConcatPlan.Action.
const (
	ConcatActionCopy      = "copy"
	ConcatActionTranscode = "transcode"
)

// ConcatMediaOptions joins several media files into one MP4.
//
// The caller states the result it wants. The library probes every input and
// stream-copies when the inputs are interchangeable, otherwise it re-encodes.
// The concat list file is created and removed by the library.
type ConcatMediaOptions struct {
	// Inputs are the files to join, in playback order.
	Inputs []string
	// OutputPath is overwritten if it exists.
	OutputPath string
	// Profile is the video target used when re-encoding. Zero value means H.264.
	Profile encode.VideoProfile
	// Width and Height scale the output when re-encoding (both > 0). Setting them
	// forces a re-encode because a stream copy cannot rescale.
	Width, Height int
	// FPS, when > 0, sets the output frame rate and forces a re-encode. Use it for
	// speed-changed timelapses, where timestamps must be rewritten.
	FPS float64
	// OnPlan, when set, is called with the plan before ffmpeg starts.
	OnPlan func(ConcatPlan)
}

// ConcatPlan is the chosen concat strategy.
type ConcatPlan struct {
	Action string // ConcatActionCopy or ConcatActionTranscode
	Reason string
	Inputs int
	Probed bool
}

// String is a one-line summary suitable for logs.
func (p ConcatPlan) String() string {
	return fmt.Sprintf("action=%s inputs=%d probed=%t: %s", p.Action, p.Inputs, p.Probed, p.Reason)
}

// DecideConcatPlan picks copy or re-encode from the probed inputs. It is pure.
//
// Copy is chosen only when every input was probed, no rescale or frame-rate change
// was requested, and every input has the same video codec, audio codec, dimensions
// and stream presence. Audio sample rate is not compared because ffprobe output
// used here does not carry it; mismatched sample rates are a known limitation of
// the copy path, so such inputs should be re-encoded by the caller with FPS set.
func DecideConcatPlan(opts ConcatMediaOptions, facts []SourceFacts) ConcatPlan {
	plan := ConcatPlan{Inputs: len(opts.Inputs), Probed: true}
	for _, f := range facts {
		if !f.Probed {
			plan.Probed = false
		}
	}
	switch {
	case opts.FPS > 0:
		plan.Action, plan.Reason = ConcatActionTranscode, "frame rate change requested"
	case opts.Width > 0 && opts.Height > 0:
		plan.Action, plan.Reason = ConcatActionTranscode, "rescale requested"
	case !plan.Probed:
		plan.Action, plan.Reason = ConcatActionTranscode, "an input could not be probed"
	case len(facts) == 0:
		plan.Action, plan.Reason = ConcatActionTranscode, "no inputs"
	default:
		for i, f := range facts {
			// Missing frame rate or time base means the demuxer cannot be trusted to
			// line the inputs up, so re-encode rather than copy.
			if (f.HasVideo && (f.VideoCodec == "" || f.FrameRate == "" || f.TimeBase == "")) ||
				(f.HasAudio && f.AudioCodec == "") {
				plan.Action = ConcatActionTranscode
				plan.Reason = fmt.Sprintf("input %d has incomplete stream facts", i)
				return plan
			}
		}
		first := facts[0]
		for i, f := range facts[1:] {
			if f != first {
				plan.Action = ConcatActionTranscode
				plan.Reason = fmt.Sprintf("input %d stream parameters differ from input 0", i+1)
				return plan
			}
		}
		plan.Action, plan.Reason = ConcatActionCopy, "all inputs share codecs, dimensions and stream layout"
	}
	return plan
}

// validateConcatOptions rejects scale and rate settings that cannot be applied.
// Width and Height must be set together, and nothing may be negative.
func validateConcatOptions(opts ConcatMediaOptions) error {
	if opts.Width < 0 || opts.Height < 0 {
		return fmt.Errorf("concat media: Width and Height must not be negative")
	}
	if (opts.Width > 0) != (opts.Height > 0) {
		return fmt.Errorf("concat media: set both Width and Height to rescale, or neither")
	}
	if opts.FPS < 0 {
		return fmt.Errorf("concat media: FPS must not be negative")
	}
	return nil
}

// BuildConcatList renders a concat demuxer list. Single quotes in paths are
// escaped with the demuxer's '\” idiom.
func BuildConcatList(paths []string) string {
	var b strings.Builder
	for _, p := range paths {
		b.WriteString("file '")
		b.WriteString(strings.ReplaceAll(p, "'", `'\''`))
		b.WriteString("'\n")
	}
	return b.String()
}

// BuildConcatMediaArgs returns the ffmpeg argv for a resolved plan. listPath is
// the concat list written by ConcatMedia.
func BuildConcatMediaArgs(runner *ffexec.Runner, caps *capabilities.Capabilities, opts ConcatMediaOptions, plan ConcatPlan, listPath string) ([]string, error) {
	args := []string{"-hide_banner", "-nostats"}
	args = runner.AppendProtocolWhitelistArgs(args)
	args = append(args, "-f", "concat", "-safe", "0", "-i", listPath)
	if plan.Action == ConcatActionCopy {
		return append(args, "-c", "copy", "-y", opts.OutputPath), nil
	}

	profile := opts.Profile
	if profile.Codec == "" {
		profile.Codec = encode.CodecH264
	}
	resolver := encode.NewResolver(caps)
	vidArgs, err := resolver.VideoEncoderArgs(profile)
	if err != nil {
		return nil, err
	}
	if opts.Width > 0 && opts.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", opts.Width, opts.Height))
	}
	if opts.FPS > 0 {
		args = append(args, "-r", fmt.Sprintf("%.3f", opts.FPS))
	}
	// vidArgs already carries the pixel format chosen for the encoder (for example
	// nv12 for QSV), so it is not overridden here.
	args = append(args, vidArgs...)
	args = append(args, "-c:a", "aac")
	return append(args, "-y", opts.OutputPath), nil
}

// ConcatMedia joins opts.Inputs into opts.OutputPath, stream-copying when the
// inputs match and re-encoding otherwise (see DecideConcatPlan).
func ConcatMedia(ctx context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts ConcatMediaOptions) (ConcatPlan, error) {
	if len(opts.Inputs) == 0 {
		return ConcatPlan{}, fmt.Errorf("concat media: no inputs")
	}
	if err := validateConcatOptions(opts); err != nil {
		return ConcatPlan{}, err
	}
	facts := make([]SourceFacts, 0, len(opts.Inputs))
	for _, in := range opts.Inputs {
		info, err := probe.ProbeStream(ctx, runner, probe.ProbeStreamOptions{URL: in, StreamType: probe.StreamFile})
		facts = append(facts, SourceFactsFromProbe(info, err))
	}
	plan := DecideConcatPlan(opts, facts)

	absInputs := make([]string, 0, len(opts.Inputs))
	for _, in := range opts.Inputs {
		abs, err := filepath.Abs(in)
		if err != nil {
			return plan, err
		}
		absInputs = append(absInputs, abs)
	}
	list, err := os.CreateTemp(filepath.Dir(opts.OutputPath), ".concat-*.txt")
	if err != nil {
		return plan, fmt.Errorf("concat media: create list: %w", err)
	}
	defer os.Remove(list.Name())
	if _, err := list.WriteString(BuildConcatList(absInputs)); err != nil {
		list.Close()
		return plan, fmt.Errorf("concat media: write list: %w", err)
	}
	if err := list.Close(); err != nil {
		return plan, fmt.Errorf("concat media: close list: %w", err)
	}

	args, err := BuildConcatMediaArgs(runner, caps, opts, plan, list.Name())
	if err != nil {
		return plan, err
	}
	if opts.OnPlan != nil {
		opts.OnPlan(plan)
	}
	_, err = runner.RunFFmpeg(ctx, args...)
	return plan, err
}

package ffmpeg

import (
	"context"

	"github.com/gtsteffaniak/go-ffmpeg/concurrency"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
)

// ConcatMediaOptions configures ConcatMedia.
type ConcatMediaOptions = ops.ConcatMediaOptions

// ConcatPlan describes the strategy ConcatMedia chose (copy or re-encode).
type ConcatPlan = ops.ConcatPlan

// SegmentPlan describes the strategy SegmentRecord chose for video and audio.
type SegmentPlan = ops.SegmentPlan

// RecordMode states the intent for video in SegmentRecord.
type RecordMode = ops.RecordMode

// Recording modes for SegmentRecordOptions.Mode.
const (
	RecordTranscode = ops.RecordTranscode
	RecordCopy      = ops.RecordCopy
	RecordAuto      = ops.RecordAuto
)

// ConcatMedia joins inputs into OutputPath. It stream-copies when the inputs are
// interchangeable and re-encodes otherwise; the returned plan says which and why.
func (s *Service) ConcatMedia(ctx context.Context, opts ConcatMediaOptions) (ConcatPlan, error) {
	if err := s.ensureDetected(ctx); err != nil {
		return ConcatPlan{}, err
	}
	if err := s.require("ConcatMedia"); err != nil {
		return ConcatPlan{}, err
	}
	// The profile only matters when the plan may re-encode.
	if opts.FPS > 0 || (opts.Width > 0 && opts.Height > 0) {
		if err := s.ValidateVideoProfile(opts.Profile); err != nil {
			return ConcatPlan{}, err
		}
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	var plan ConcatPlan
	err := s.runWithClass(ctx, concurrency.SlotEncode, "", func() error {
		var runErr error
		plan, runErr = ops.ConcatMedia(ctx, s.runner, caps, opts)
		return wrapOp("ConcatMedia", ErrEncodeFailed, runErr)
	})
	return plan, err
}

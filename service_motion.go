package ffmpeg

import (
	"context"

	"github.com/gtsteffaniak/go-ffmpeg/concurrency"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// MotionCrop re-export: normalized 0–1 rectangle restricting motion analysis.
type MotionCrop = ops.MotionCrop

// MotionDetectOptions re-export.
type MotionDetectOptions = ops.MotionDetectOptions

// MotionSample re-export.
type MotionSample = ops.MotionSample

// MotionEvent re-export.
type MotionEvent = ops.MotionEvent

// MotionResult re-export.
type MotionResult = ops.MotionResult

// MotionDefaults re-export.
type MotionDefaults = ops.MotionDefaults

// InputSource re-export is declared elsewhere; DetectMotion takes InputSource.
//
// DetectMotion runs one frame-difference analysis pass over a file (or bounded
// stream window) and returns merged motion events. It decodes; it never encodes.
func (s *Service) DetectMotion(ctx context.Context, opts MotionDetectOptions) (MotionResult, error) {
	if err := s.ensureDetected(ctx); err != nil {
		return MotionResult{}, err
	}
	if err := s.require("DetectMotion"); err != nil {
		return MotionResult{}, err
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()

	dur := opts.DurationSec
	if dur <= 0 {
		err := s.runWithClass(ctx, concurrency.SlotProbe, opts.Input.URL, func() error {
			var probeErr error
			dur, probeErr = probe.GetMediaDuration(ctx, s.runner, opts.Input.URL)
			return probeErr
		})
		if err != nil {
			return MotionResult{}, err
		}
	}
	opts.DurationSec = dur

	var result MotionResult
	err := s.runWithClass(ctx, concurrency.SlotDecode, opts.Input.URL, func() error {
		var opErr error
		result, opErr = ops.DetectMotion(ctx, s.runner, caps, opts)
		return opErr
	})
	if err != nil {
		return MotionResult{}, wrapOp("DetectMotion", ErrEncodeFailed, err)
	}
	return result, nil
}

package ffmpeg

import (
	"context"
	"io"

	"github.com/gtsteffaniak/go-ffmpeg/concurrency"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// CaptureAccuracy re-export.
type CaptureAccuracy = ops.CaptureAccuracy

const (
	CaptureFast  = ops.CaptureFast
	CaptureExact = ops.CaptureExact
)

// FrameFormat re-export.
type FrameFormat = ops.FrameFormat

const (
	FrameJPEG = ops.FrameJPEG
	FrameWebP = ops.FrameWebP
	FramePNG  = ops.FramePNG
)

// FramePosition re-export.
type FramePosition = ops.FramePosition

// AtSec and AtPercent construct frame positions.
var (
	AtSec     = ops.AtSec
	AtPercent = ops.AtPercent
)

// CaptureLightProbe re-export.
type CaptureLightProbe = ops.CaptureLightProbe

// FrameOptions configures single-frame capture.
type FrameOptions = ops.FrameOptions

// FrameCaptureValidationError reports invalid capture options.
type FrameCaptureValidationError = ops.FrameCaptureValidationError

// RTSPInputOptions adds RTSP-specific input flags.
type RTSPInputOptions = ops.RTSPInputOptions

// CaptureFrame streams one encoded frame to w.
func (s *Service) CaptureFrame(ctx context.Context, w io.Writer, opts FrameOptions) error {
	if err := s.ensureDetected(ctx); err != nil {
		return err
	}
	if err := s.require("CaptureFrame"); err != nil {
		return err
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	if err := ops.ValidateFrameOptions(caps, opts); err != nil {
		return err
	}

	dur := opts.DurationSec
	if ops.FrameNeedsDurationProbe(opts.At, dur) {
		err := s.runWithClass(ctx, concurrency.SlotProbe, opts.Input.URL, func() error {
			var probeErr error
			dur, probeErr = probe.GetMediaDuration(ctx, s.runner, opts.Input.URL)
			return probeErr
		})
		if err != nil {
			return err
		}
	}
	seekSec := ops.ResolveFrameSeekSec(opts.At, dur)

	return s.runWithClass(ctx, concurrency.SlotDecode, opts.Input.URL, func() error {
		return wrapOp("CaptureFrame", ErrEncodeFailed, ops.CaptureFrame(ctx, s.runner, caps, w, opts, seekSec))
	})
}

// CaptureFrameFile writes one frame to path atomically (temp file + rename).
func (s *Service) CaptureFrameFile(ctx context.Context, path string, opts FrameOptions) error {
	if err := s.ensureDetected(ctx); err != nil {
		return err
	}
	if err := s.require("CaptureFrame"); err != nil {
		return err
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	if err := ops.ValidateFrameOptions(caps, opts); err != nil {
		return err
	}

	dur := opts.DurationSec
	if ops.FrameNeedsDurationProbe(opts.At, dur) {
		err := s.runWithClass(ctx, concurrency.SlotProbe, opts.Input.URL, func() error {
			var probeErr error
			dur, probeErr = probe.GetMediaDuration(ctx, s.runner, opts.Input.URL)
			return probeErr
		})
		if err != nil {
			return err
		}
	}
	seekSec := ops.ResolveFrameSeekSec(opts.At, dur)

	return s.runWithClass(ctx, concurrency.SlotDecode, opts.Input.URL, func() error {
		return wrapOp("CaptureFrame", ErrEncodeFailed, ops.CaptureFrameFile(ctx, s.runner, caps, path, opts, seekSec))
	})
}

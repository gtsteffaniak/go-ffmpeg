package ffmpeg

import (
	"context"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

// ReProbeHardware re-runs layered hardware encoder detection and refreshes the codec matrix.
// Pass no encoder names to re-probe every known hardware encoder.
func (s *Service) ReProbeHardware(ctx context.Context, encoders ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.caps == nil || s.runner == nil {
		return ErrNotDetected
	}
	plat := s.caps.HardwarePlatform
	if plat.OS == "" {
		plat = s.caps.Platform
	}
	hierarchy := s.caps.EncoderHierarchy
	if len(hierarchy) == 0 {
		hierarchy = capabilities.HierarchyForPlatform(plat)
	}
	gpuScoped := s.caps.SelectedGPU != nil && s.caps.SelectedGPU.Enabled && !s.caps.SelectedGPU.SoftwareOnly
	if err := capabilities.ReProbeHardwareEncoders(ctx, s.caps, s.runner, plat, hierarchy, gpuScoped, encoders...); err != nil {
		return err
	}
	s.resolver = encode.NewResolver(s.caps)
	WithGroup(s.cfg.Logger, "ffmpeg").Info(s.caps.HardwareProbeSummaryLine())
	return nil
}

// MarkHardwareEncoderFailed records a runtime hardware failure, marks the encoder unavailable,
// and returns the structured probe result (call ReProbeHardware later to retry).
func (s *Service) MarkHardwareEncoderFailed(encoder string, stderr string) capabilities.EncoderProbeResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.caps == nil {
		return capabilities.EncoderProbeResult{Encoder: encoder}
	}
	probe := s.caps.MarkEncoderUnhealthy(encoder, stderr)
	hierarchy := s.caps.EncoderHierarchy
	if len(hierarchy) == 0 {
		hierarchy = capabilities.HierarchyForPlatform(s.caps.Platform)
	}
	capabilities.BuildCodecMatrix(s.caps, hierarchy)
	if s.resolver != nil {
		s.resolver = encode.NewResolver(s.caps)
	}
	WithGroup(s.cfg.Logger, "ffmpeg").Warn(
		"hardware encoder marked unhealthy: " + encoder + " (" + string(probe.Code) + "): " + probe.Hint,
	)
	return probe
}

// NotifyHardwareEncodeFailure classifies stderr from a failed transcode and marks the encoder unhealthy when appropriate.
func (s *Service) NotifyHardwareEncodeFailure(encoder, stderr string) {
	if encoder == "" {
		return
	}
	classified := encode.FailureClassifier{}.Classify(stderr)
	if classified.Kind != encode.FailureHardware {
		return
	}
	s.MarkHardwareEncoderFailed(encoder, stderr)
}

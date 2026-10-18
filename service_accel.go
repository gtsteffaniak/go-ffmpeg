package ffmpeg

import (
	"fmt"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func (s *Service) buildDetectOptions() (capabilities.DetectOptions, error) {
	opts := capabilities.DetectOptions{SkipHWTests: s.cfg.SkipHWTests}
	if len(s.cfg.EncoderHierarchy) > 0 {
		opts.EncoderHierarchy = s.cfg.EncoderHierarchy
	}
	if pref, err := capabilities.ParseEncoderPreference(s.cfg.EncoderPreference); err == nil {
		opts.EncoderPreference = pref
	}

	ds := s.cfg.effectiveDeviceSelection()
	strict := s.cfg.effectiveGPUStrictness()
	choice, fallbacks, err := ResolveDeviceSelection(ds, strict)
	if err != nil {
		return opts, fmt.Errorf("gpu selection: %w", err)
	}
	if choice.Warning != "" && s.cfg.Logger != nil {
		s.cfg.Logger.Warn(choice.Warning, "gpu", ds.Primary.String())
	}
	opts.GPU = choice
	opts.DeviceChain = append([]platform.GPUChoice{choice}, fallbacks...)
	if choice.RenderDevice != "" {
		opts.RenderDevice = choice.RenderDevice
	}
	if s.cfg.AllowSoftwareEncoderFallback != nil {
		opts.AllowSoftware = s.cfg.AllowSoftwareEncoderFallback
	}
	if choice.SoftwareOnly {
		opts.SkipHWTests = true
		if len(opts.EncoderHierarchy) == 0 {
			opts.EncoderHierarchy = []capabilities.AccelType{capabilities.AccelNone}
		}
	}
	return opts, nil
}

func (s *Service) gpuChoice() (platform.GPUChoice, error) {
	if s == nil {
		return platform.GPUChoice{}, nil
	}
	sel := s.cfg.effectiveGPUSelector()
	return ResolveGPUSelector(sel, s.cfg.effectiveGPUStrictness())
}

// GPUSelector returns the configured typed GPU selection.
func (s *Service) GPUSelector() GPUSelector {
	if s == nil {
		return GPUSelector{Kind: GPUAuto}
	}
	return s.cfg.effectiveGPUSelector()
}

// GPU returns the deprecated string GPU config (empty when unset).
func (s *Service) GPU() string {
	if s == nil {
		return ""
	}
	return s.cfg.GPU
}

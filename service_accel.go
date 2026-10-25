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
	mode, err := capabilities.ParseVulkanEncoderMode(s.cfg.VulkanEncoder)
	if err != nil {
		return opts, fmt.Errorf("VulkanEncoder: %w", err)
	}
	opts.VulkanEncoder = mode

	strict := s.cfg.effectiveGPUStrictness()
	chain, orderSummary, err := ResolveConfigDeviceChain(s.cfg, strict, platform.DefaultGPUResolveEnv())
	if err != nil {
		return opts, fmt.Errorf("gpu selection: %w", err)
	}
	var choice platform.GPUChoice
	if len(chain) > 0 {
		choice = chain[0]
	}
	if choice.Warning != "" && s.cfg.Logger != nil {
		s.cfg.Logger.Warn(choice.Warning, "gpu", choice.Label)
	}
	opts.GPU = choice
	opts.DeviceChain = chain
	opts.DeviceOrderSummary = orderSummary
	opts.DevicePreference = s.cfg.effectiveDevicePreference().String()
	if choice.RenderDevice != "" {
		opts.RenderDevice = choice.RenderDevice
	}
	if s.cfg.AllowSoftwareEncoderFallback != nil {
		opts.AllowSoftware = s.cfg.AllowSoftwareEncoderFallback
	}
	if choice.SoftwareOnly && len(opts.EncoderHierarchy) == 0 {
		// Software selection steers preference only. Smoke tests still run so
		// hosts can show what the machine could do and switch at runtime.
		opts.EncoderHierarchy = []capabilities.AccelType{capabilities.AccelNone}
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

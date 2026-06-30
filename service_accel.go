package ffmpeg

import (
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func (s *Service) buildDetectOptions() capabilities.DetectOptions {
	opts := capabilities.DetectOptions{SkipHWTests: s.cfg.SkipHWTests}
	if len(s.cfg.EncoderHierarchy) > 0 {
		opts.EncoderHierarchy = s.cfg.EncoderHierarchy
	}
	if s.cfg.GPU == "" {
		return opts
	}
	choice, err := capabilities.ResolveGPUChoice(s.cfg.GPU)
	if err != nil || !choice.Enabled {
		return opts
	}
	opts.GPU = choice
	opts.RenderDevice = choice.RenderDevice
	if len(opts.EncoderHierarchy) == 0 {
		plat := platform.Detect()
		opts.EncoderHierarchy = capabilities.HierarchyForGPU(choice.Vendor, plat)
	}
	return opts
}

func (s *Service) gpuChoice() (platform.GPUChoice, error) {
	if s == nil || s.cfg.GPU == "" {
		return platform.GPUChoice{}, nil
	}
	return capabilities.ResolveGPUChoice(s.cfg.GPU)
}

// GPU returns the configured gpu selection string (empty means software only).
func (s *Service) GPU() string {
	if s == nil {
		return ""
	}
	return s.cfg.GPU
}

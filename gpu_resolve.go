package ffmpeg

import (
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// ResolveGPUSelector resolves a typed selector on this host.
func ResolveGPUSelector(sel GPUSelector, strict GPUStrictness) (platform.GPUChoice, error) {
	req := platform.GPURequest{
		Kind:       selectionKindFrom(sel.Kind),
		RenderNode: sel.RenderNode,
		NameMatch:  sel.NameMatch,
		Label:      sel.String(),
	}
	return platform.ResolveGPURequestDefaultEnv(req, strict == GPUStrictOnUnavailable)
}

// ResolveGPUSelectorWithEnv resolves using an injected environment (tests).
func ResolveGPUSelectorWithEnv(sel GPUSelector, strict GPUStrictness, env platform.GPUResolveEnv) (platform.GPUChoice, error) {
	req := platform.GPURequest{
		Kind:       selectionKindFrom(sel.Kind),
		RenderNode: sel.RenderNode,
		NameMatch:  sel.NameMatch,
		Label:      sel.String(),
	}
	return platform.ResolveGPURequest(req, env, strict == GPUStrictOnUnavailable)
}

func selectionKindFrom(k GPUKind) platform.SelectionKind {
	switch k {
	case GPUIntegrated:
		return platform.SelectionIntegrated
	case GPUDiscrete:
		return platform.SelectionDiscrete
	case GPUSoftware:
		return platform.SelectionSoftware
	case GPUDevice:
		return platform.SelectionDevice
	default:
		return platform.SelectionAuto
	}
}

package platform

// ScopedPlatform narrows platform gates to a selected GPU vendor.
// Used when detection runs for a specific gpu config value.
func ScopedPlatform(base Info, choice GPUChoice) Info {
	if !choice.Enabled {
		return base
	}
	scoped := base
	if base.Details != nil {
		scoped.Details = make(map[string]string, len(base.Details)+3)
		for k, v := range base.Details {
			scoped.Details[k] = v
		}
	} else {
		scoped.Details = map[string]string{}
	}
	scoped.Details["selected_gpu"] = choice.Name
	scoped.Details["selected_gpu_vendor"] = choice.Vendor
	if choice.RenderDevice != "" {
		scoped.Details["render_device"] = choice.RenderDevice
	}

	switch choice.Vendor {
	case "nvidia":
		scoped.Intel = false
		scoped.AMD = false
		scoped.QSV = false
		scoped.QSVRuntime = false
	case "intel":
		scoped.NVIDIA = false
		scoped.AMD = false
	case "amd":
		scoped.NVIDIA = false
		scoped.Intel = false
		scoped.QSV = false
		scoped.QSVRuntime = false
	case "apple":
		scoped.NVIDIA = false
		scoped.Intel = false
		scoped.AMD = false
		scoped.QSV = false
		scoped.QSVRuntime = false
		scoped.VAAPI = false
		scoped.D3D12 = false
	}
	return scoped
}

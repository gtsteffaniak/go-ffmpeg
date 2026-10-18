package capabilities

import (
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// PlatformForEncoderKind returns host platform info tuned for smoke-testing a backend
// on the correct GPU device. Detection always uses the full hardware picture; this
// only picks render nodes / adapter indices for per-backend tests.
func PlatformForEncoderKind(base platform.Info, kind string, gpus []platform.GPUDevice) platform.Info {
	plat := base
	if plat.Details == nil {
		plat.Details = map[string]string{}
	} else {
		dup := make(map[string]string, len(plat.Details)+2)
		for k, v := range plat.Details {
			dup[k] = v
		}
		plat.Details = dup
	}
	switch kind {
	case "nvenc":
		if g := platform.PickNVIDIAGPUDevice(gpus); g != nil {
			plat.Details["smoke_gpu"] = g.Name
			if g.RenderNode != "" {
				plat.Details["render_device"] = g.RenderNode
			}
			plat.Details["dxgi_index"] = itoa(g.DXGIIndex)
			if g.Pinning.NVENC == platform.PinningExact || g.Pinning.CUDA == platform.PinningExact {
				plat.Details["cuda_index"] = itoa(g.CUDAIndex)
			}
		}
	case "amf":
		if g := platform.PickAMDGPUDevice(gpus); g != nil {
			plat.Details["smoke_gpu"] = g.Name
			plat.Details["dxgi_index"] = itoa(g.DXGIIndex)
		}
	case "qsv", "vaapi":
		if g := platform.PickIntelIGPUDevice(gpus); g != nil && g.RenderNode != "" {
			plat.Details["render_device"] = g.RenderNode
			plat.Details["smoke_gpu"] = g.Name
		}
	case "v4l2m2m":
		if devs := platform.ListV4L2M2MDevices(); len(devs) > 0 {
			plat.Details["v4l2_device"] = devs[0].Path
		}
	}
	return plat
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

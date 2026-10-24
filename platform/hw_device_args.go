package platform

import "fmt"

// FFmpegHWDeviceInit returns ffmpeg arguments before -i to pin a backend to choice's adapter/render node.
func FFmpegHWDeviceInit(backend string, choice GPUChoice, plat Info) []string {
	pinned := choice.DeviceID != "" || choice.Name != ""
	switch backend {
	case "nvenc":
		return nil
	case "amf", "d3d11va":
		if plat.OS == "windows" && pinned {
			idx := choice.DXGIIndex
			return []string{
				"-init_hw_device", fmt.Sprintf("d3d11va=dx:%d", idx),
				"-filter_hw_device", "dx",
			}
		}
	case "qsv":
		if plat.OS == "windows" && pinned {
			return []string{
				"-init_hw_device", fmt.Sprintf("qsv=hw,child_device=%d", choice.DXGIIndex),
				"-filter_hw_device", "hw",
			}
		}
	case "vaapi":
		dev := choice.RenderDevice
		if dev == "" && plat.Details != nil {
			dev = plat.Details["render_device"]
		}
		if dev != "" {
			return []string{
				"-init_hw_device", "vaapi=va:" + dev,
				"-filter_hw_device", "va",
			}
		}
	case "vulkan":
		return FFmpegVulkanDeviceInit(choice, plat)
	}
	return nil
}

// FFmpegVulkanDeviceInit returns ffmpeg arguments to create a Vulkan hwdevice.
// On Linux the render node path pins the device; on Windows the default adapter is used.
func FFmpegVulkanDeviceInit(choice GPUChoice, plat Info) []string {
	dev := choice.RenderDevice
	if dev == "" && plat.Details != nil {
		dev = plat.Details["render_device"]
	}
	if plat.OS == "linux" && dev != "" {
		return []string{
			"-init_hw_device", "vulkan=vk:" + dev,
			"-filter_hw_device", "vk",
		}
	}
	return []string{
		"-init_hw_device", "vulkan=vk",
		"-filter_hw_device", "vk",
	}
}

// FFmpegEncoderDeviceArgs returns encoder-scoped device arguments (after -i, before -c:v).
func FFmpegEncoderDeviceArgs(backend string, choice GPUChoice, plat Info) []string {
	switch backend {
	case "nvenc":
		if choice.Pinning.NVENC != "" || choice.Pinning.CUDA != "" {
			return []string{"-gpu", fmt.Sprintf("%d", choice.CUDAIndex)}
		}
	}
	return nil
}

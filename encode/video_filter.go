package encode

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

const videoToolboxDeviceAlias = "vt"

// VideoFilterArgs returns -filter_hw_device and -vf for hardware encoders.
// Must be appended after input and before encoder args.
func (r *Resolver) VideoFilterArgs(profile VideoProfile, decode VideoDecodeProfile, maxHeight int) ([]string, error) {
	if profile.Codec == CodecCopy {
		return nil, nil
	}
	if profile.ForceSoftware {
		return r.softwareVideoFilters(decode, maxHeight)
	}
	sel, err := r.ResolveEncoder(profile)
	if err != nil {
		return nil, err
	}
	switch sel.Accel {
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return r.vaapiVideoFilters(decode, maxHeight)
	case capabilities.AccelNVENC:
		return r.nvencVideoFilters(decode, maxHeight)
	case capabilities.AccelQSV:
		return r.qsvVideoFilters(decode, maxHeight)
	case capabilities.AccelVulkan:
		return r.vulkanVideoFilters(decode, maxHeight)
	case capabilities.AccelVideoToolbox:
		return r.videotoolboxVideoFilters(decode, maxHeight)
	default:
		return r.softwareVideoFilters(decode, maxHeight)
	}
}

func videoFilterScaleOnly(maxHeight int) ([]string, error) {
	if maxHeight <= 0 {
		return []string{"-vf", "format=yuv420p"}, nil
	}
	return []string{"-vf", fmt.Sprintf("scale=-2:min(%d\\,ih),format=yuv420p", maxHeight)}, nil
}

// decodeEmitsHardwareFrames reports whether VideoDecoderArgs will keep decoded
// frames in a hardware pixel format (cuda, vaapi, vulkan, videotoolbox). When
// true, a software filter graph must download the frames first with hwdownload,
// or ffmpeg fails with "Impossible to convert between the formats ... src: cuda".
//
// QSV decode keeps frames in system memory (its decoder args set no
// -hwaccel_output_format), and NVENC keeps CUDA surfaces only when scale_cuda
// exists (see VideoDecoderArgs).
func (r *Resolver) decodeEmitsHardwareFrames(decode VideoDecodeProfile) bool {
	switch r.decodeAccel(decode) {
	case capabilities.AccelVAAPI, capabilities.AccelD3D12, capabilities.AccelVulkan, capabilities.AccelVideoToolbox:
		return true
	case capabilities.AccelNVENC:
		return r.Caps.FilterAvailable("scale_cuda")
	default:
		return false
	}
}

// softwareVideoFilters builds the graph for a software encoder. Hardware-decoded
// frames are downloaded to system memory first so the software scale/format
// chain can consume them; the trailing format=yuv420p gives libx264 its
// expected pixel format and converts 10-bit downloads.
func (r *Resolver) softwareVideoFilters(decode VideoDecodeProfile, maxHeight int) ([]string, error) {
	if !r.decodeEmitsHardwareFrames(decode) {
		return videoFilterScaleOnly(maxHeight)
	}
	parts := []string{"hwdownload", hwDownloadFormat(decode)}
	if maxHeight > 0 {
		parts = append(parts, fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight))
	}
	parts = append(parts, "format=yuv420p")
	return []string{"-vf", strings.Join(parts, ",")}, nil
}

// scaleVTFilter caps the output height while preserving aspect ratio.
// scale_vt does not implement the "-2" width shorthand (it treats it as a literal
// negative width, producing "Picture size 4294967294xNNNN is invalid"), so the
// even width has to be computed explicitly.
func scaleVTFilter(maxHeight int) string {
	return fmt.Sprintf(
		"scale_vt=w=trunc(iw*min(%d\\,ih)/ih/2)*2:h=min(%d\\,ih)",
		maxHeight, maxHeight,
	)
}

func (r *Resolver) vulkanVideoFilters(decode VideoDecodeProfile, maxHeight int) ([]string, error) {
	choice := platform.GPUChoice{}
	if r != nil && r.Caps != nil && r.Caps.SelectedGPU != nil {
		choice = *r.Caps.SelectedGPU
	}
	plat := platform.Info{}
	if r != nil && r.Caps != nil {
		plat = r.Caps.Platform
	}
	useScaleVulkan := r != nil && r.Caps != nil && r.Caps.FilterAvailable("scale_vulkan")
	var decodeSel capabilities.DecoderSelection
	if r != nil {
		decodeSel, _ = r.ResolveDecoder(decode)
	}
	vkDecode := decodeSel.Accel == capabilities.AccelVulkan
	otherHWDecode := decodeSel.Accel != capabilities.AccelNone && !vkDecode && !decode.ForceSoftware

	var parts []string
	switch {
	case vkDecode && useScaleVulkan && maxHeight > 0:
		parts = append(parts, fmt.Sprintf("scale_vulkan=w=trunc(iw*min(%d\\,ih)/ih/2)*2:h=min(%d\\,ih)", maxHeight, maxHeight))
	case vkDecode && maxHeight > 0:
		parts = append(parts, "hwdownload", fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight), "format=nv12", "hwupload")
	case useScaleVulkan && maxHeight > 0:
		if otherHWDecode {
			parts = append(parts, "hwdownload")
		}
		parts = append(parts, "format=nv12", "hwupload", fmt.Sprintf("scale_vulkan=w=trunc(iw*min(%d\\,ih)/ih/2)*2:h=min(%d\\,ih)", maxHeight, maxHeight))
	default:
		if otherHWDecode {
			parts = append(parts, "hwdownload")
		}
		if maxHeight > 0 {
			parts = append(parts, fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight))
		}
		parts = append(parts, "format=nv12", "hwupload")
	}
	args := []string{"-filter_hw_device", "vk", "-vf", strings.Join(parts, ",")}
	if !vkDecode {
		init := platform.FFmpegVulkanDeviceInit(choice, plat)
		args = append(init, args...)
	}
	return args, nil
}

func decodeUsesVulkan(r *Resolver, decode VideoDecodeProfile) bool {
	if decode.ForceSoftware {
		return false
	}
	if r == nil {
		return false
	}
	sel, err := r.ResolveDecoder(decode)
	if err != nil {
		return false
	}
	return sel.Accel == capabilities.AccelVulkan
}

// decodeAccel returns the hardware decode backend that [Resolver.VideoDecoderArgs]
// will use for decode, or [capabilities.AccelNone] for software decode.
func (r *Resolver) decodeAccel(decode VideoDecodeProfile) capabilities.AccelType {
	if decode.ForceSoftware || r == nil || r.Caps == nil {
		return capabilities.AccelNone
	}
	sel, err := r.ResolveDecoder(decode)
	if err != nil || sel.Decoder == "" {
		return capabilities.AccelNone
	}
	return sel.Accel
}

// hwDownloadFormat returns the format filter that pins hwdownload's output to
// the decoded surface's sw_format: p010le for sources deeper than 8 bits, nv12
// otherwise. hwdownload only transfers to the frame context's own format, and a
// list such as "nv12|p010le" lets negotiation pick nv12 for 10-bit surfaces,
// which fails with "Invalid output format nv12 for hwframe download".
func hwDownloadFormat(decode VideoDecodeProfile) string {
	if decode.BitDepth > 8 {
		return "format=p010le"
	}
	return "format=nv12"
}

func (r *Resolver) vaapiVideoFilters(decode VideoDecodeProfile, maxHeight int) ([]string, error) {
	accel := r.decodeAccel(decode)
	hwDecode := accel == capabilities.AccelVAAPI || accel == capabilities.AccelD3D12

	var parts []string
	switch {
	case hwDecode && r.Caps.FilterAvailable("scale_vaapi"):
		// Frames are VAAPI surfaces (-hwaccel_output_format vaapi). Scale and
		// convert (10-bit -> nv12) on the GPU; software filters cannot take them.
		if maxHeight > 0 {
			parts = append(parts, fmt.Sprintf("scale_vaapi=w=-2:h=min(%d\\,ih):format=nv12", maxHeight))
		} else {
			parts = append(parts, "scale_vaapi=format=nv12")
		}
	case hwDecode:
		parts = append(parts, "hwdownload", hwDownloadFormat(decode))
		if maxHeight > 0 {
			parts = append(parts, fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight))
		}
		parts = append(parts, "format=nv12", "hwupload")
	default:
		if maxHeight > 0 {
			parts = append(parts, fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight))
		}
		parts = append(parts, "format=nv12", "hwupload")
	}
	args := []string{"-filter_hw_device", "va", "-vf", strings.Join(parts, ",")}
	if !hwDecode {
		if r == nil || r.Caps == nil {
			return args, nil
		}
		renderDev := platform.RenderDevice(r.Caps.Platform.Details)
		init := []string{"-init_hw_device", "vaapi=va:" + renderDev}
		args = append(init, args...)
	}
	return args, nil
}

// nvencVideoFilters builds the filter graph for NVENC.
//
// With CUDA decode and scale_cuda available, [Resolver.VideoDecoderArgs] keeps
// frames on the GPU (-hwaccel_output_format cuda). They are scaled and converted
// to nv12 on the GPU, then downloaded as software nv12 frames: hwdownload is
// pinned to nv12 (the sw_format scale_cuda just produced), so there is no
// format negotiation ambiguity, and the encoder keeps receiving software frames
// as before (so the trailing -pix_fmt yuv420p stays valid).
//
// Without scale_cuda the decoder args omit the CUDA output format, so frames are
// already in system memory and the plain software graph applies.
func (r *Resolver) nvencVideoFilters(decode VideoDecodeProfile, maxHeight int) ([]string, error) {
	if r.decodeAccel(decode) != capabilities.AccelNVENC || !r.Caps.FilterAvailable("scale_cuda") {
		// NVENC encode with non-NVENC decode, or CUDA decode without scale_cuda
		// (frames already in system memory): the software graph applies, but a
		// hardware decode backend (vaapi/vulkan/videotoolbox) still needs the
		// frames downloaded first.
		return r.softwareVideoFilters(decode, maxHeight)
	}
	var parts []string
	if maxHeight > 0 {
		parts = append(parts, fmt.Sprintf("scale_cuda=w=-2:h=min(%d\\,ih):format=nv12", maxHeight))
	} else {
		parts = append(parts, "scale_cuda=format=nv12")
	}
	parts = append(parts, "hwdownload", "format=nv12")
	return []string{"-vf", strings.Join(parts, ",")}, nil
}

func (r *Resolver) qsvVideoFilters(decode VideoDecodeProfile, maxHeight int) ([]string, error) {
	parts := []string{}
	// QSV decode emits system-memory frames, but another hardware backend
	// (vaapi/cuda/vulkan/videotoolbox) chosen for decode keeps them on the GPU,
	// so download before the software scale/format into QSV's nv12 input.
	if r.decodeEmitsHardwareFrames(decode) {
		parts = append(parts, "hwdownload", hwDownloadFormat(decode))
	}
	if maxHeight > 0 {
		parts = append(parts, fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight))
	}
	parts = append(parts, "format=nv12")
	return []string{"-vf", strings.Join(parts, ",")}, nil
}

func (r *Resolver) videotoolboxVideoFilters(decode VideoDecodeProfile, maxHeight int) ([]string, error) {
	useScaleVT := r != nil && r.Caps != nil && r.Caps.FilterAvailable("scale_vt")
	vtDecode := decodeUsesVideoToolbox(r, decode)

	var parts []string
	switch {
	case vtDecode && useScaleVT:
		if maxHeight > 0 {
			parts = append(parts, scaleVTFilter(maxHeight))
		}
	case vtDecode:
		if maxHeight > 0 {
			parts = append(parts, "hwdownload", fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight), "format=nv12", "hwupload")
		}
	case useScaleVT:
		parts = append(parts, "format=nv12", "hwupload")
		if maxHeight > 0 {
			parts = append(parts, scaleVTFilter(maxHeight))
		}
	default:
		if maxHeight > 0 {
			parts = append(parts, fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight))
		}
		parts = append(parts, "format=nv12", "hwupload")
	}
	if len(parts) == 0 {
		return nil, nil
	}

	args := []string{"-filter_hw_device", videoToolboxDeviceAlias, "-vf", strings.Join(parts, ",")}
	return args, nil
}

func decodeUsesVideoToolbox(r *Resolver, decode VideoDecodeProfile) bool {
	if decode.ForceSoftware {
		return false
	}
	if r == nil {
		return false
	}
	sel, err := r.ResolveDecoder(decode)
	if err != nil {
		return false
	}
	return sel.Accel == capabilities.AccelVideoToolbox
}

// EncoderUsesHardware reports whether the resolved encoder is a hardware backend.
func (r *Resolver) EncoderUsesHardware(profile VideoProfile) (bool, string, error) {
	if profile.ForceSoftware {
		return false, "libx264", nil
	}
	sel, err := r.ResolveEncoder(profile)
	if err != nil {
		return false, "", err
	}
	hw := sel.Accel != capabilities.AccelNone && sel.Accel != ""
	return hw, sel.Encoder, nil
}

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
	if profile.ForceSoftware || profile.Codec == CodecCopy {
		return videoFilterScaleOnly(maxHeight)
	}
	sel, err := r.ResolveEncoder(profile)
	if err != nil {
		return nil, err
	}
	switch sel.Accel {
	case capabilities.AccelVAAPI, capabilities.AccelD3D12:
		return r.vaapiVideoFilters(decode, maxHeight)
	case capabilities.AccelQSV:
		return qsvVideoFilters(maxHeight)
	case capabilities.AccelVideoToolbox:
		return r.videotoolboxVideoFilters(decode, maxHeight)
	default:
		return videoFilterScaleOnly(maxHeight)
	}
}

func videoFilterScaleOnly(maxHeight int) ([]string, error) {
	if maxHeight <= 0 {
		return nil, nil
	}
	return []string{"-vf", fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight)}, nil
}

func (r *Resolver) vaapiVideoFilters(decode VideoDecodeProfile, maxHeight int) ([]string, error) {
	parts := []string{}
	if maxHeight > 0 {
		parts = append(parts, fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight))
	}
	parts = append(parts, "format=nv12", "hwupload")
	args := []string{"-filter_hw_device", "va", "-vf", strings.Join(parts, ",")}
	if decode.ForceSoftware || decode.Accel == capabilities.AccelNone || decode.Accel == "" {
		if r == nil || r.Caps == nil {
			return args, nil
		}
		renderDev := platform.RenderDevice(r.Caps.Platform.Details)
		init := []string{"-init_hw_device", "vaapi=va:" + renderDev}
		args = append(init, args...)
	}
	return args, nil
}

func qsvVideoFilters(maxHeight int) ([]string, error) {
	parts := []string{}
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
			parts = append(parts, fmt.Sprintf("scale_vt=w=-2:h=min(%d\\,ih)", maxHeight))
		}
	case vtDecode:
		if maxHeight > 0 {
			parts = append(parts, "hwdownload", fmt.Sprintf("scale=-2:min(%d\\,ih)", maxHeight), "format=nv12", "hwupload")
		}
	case useScaleVT:
		parts = append(parts, "format=nv12", "hwupload")
		if maxHeight > 0 {
			parts = append(parts, fmt.Sprintf("scale_vt=w=-2:h=min(%d\\,ih)", maxHeight))
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

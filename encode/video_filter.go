package encode

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

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

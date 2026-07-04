package encode

import "github.com/gtsteffaniak/go-ffmpeg/capabilities"

// VideoToolboxPreInputInit returns -init_hw_device args that must appear before -i.
// When hardware decode already initializes the device, this returns nil to avoid duplicates.
func (r *Resolver) VideoToolboxPreInputInit(decode VideoDecodeProfile, profile VideoProfile) []string {
	if !r.videotoolboxEncodeActive(profile) {
		return nil
	}
	if decodeUsesVideoToolbox(r, decode) {
		return nil
	}
	return []string{"-init_hw_device", "videotoolbox=" + videoToolboxDeviceAlias}
}

func (r *Resolver) videotoolboxEncodeActive(profile VideoProfile) bool {
	if profile.ForceSoftware || profile.Codec == CodecCopy {
		return false
	}
	if r == nil || r.Caps == nil {
		return false
	}
	sel, err := r.ResolveEncoder(profile)
	if err != nil {
		return false
	}
	return sel.Accel == capabilities.AccelVideoToolbox
}

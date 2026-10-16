package platform

import "strings"

// PackageHints builds install/fix strings for capability reports and smoke-test errors.
type PackageHints struct {
	Distro DistroContext
}

// NewPackageHints uses detected distro when ctx is zero.
func NewPackageHints(ctx DistroContext) PackageHints {
	if ctx.Family == DistroFamilyUnknown && ctx.ID == "" {
		ctx = DetectDistro()
	}
	return PackageHints{Distro: ctx}
}

// QSVRuntimeFix returns install lines for a missing oneVPL GPU runtime.
func (h PackageHints) QSVRuntimeFix() []string {
	d := h.Distro
	pkgs := d.QSVRuntimePackages()
	if len(pkgs) == 0 {
		return []string{
			"Install the Intel oneVPL GPU runtime for your distro (Arch: vpl-gpu-rt; Debian/Ubuntu: libmfx-gen1.2; Fedora: intel-vpl-gpu-rt; openSUSE: libmfx-gen)",
			"Verify: ls /usr/lib*/libmfx-gen.so* && vainfo --display drm --device /dev/dri/renderD128",
		}
	}
	lines := []string{
		d.QSVStackDescription(),
		"libvpl is the oneVPL dispatcher; the GPU runtime package provides libmfx-gen for FFmpeg QSV",
		"Fix: " + d.InstallCommand(pkgs...) + " && sudo reboot",
		"Verify: ls /usr/lib/x86_64-linux-gnu/libmfx-gen.so* /usr/lib64/libmfx-gen.so* 2>/dev/null; vainfo --display drm --device /dev/dri/renderD128",
	}
	return lines
}

// IntelVADriverFix returns a one-line install hint for Intel VAAPI.
func (h PackageHints) IntelVADriverFix() string {
	pkgs := h.Distro.IntelVADriverPackages()
	if len(pkgs) == 0 {
		return "install the Intel VAAPI driver for your distro"
	}
	return h.Distro.InstallCommand(pkgs...)
}

// VAAPIInitFix returns hints when VAAPI fails to initialize.
func (h PackageHints) VAAPIInitFix(intel bool) []string {
	if intel {
		return []string{
			"Fix: " + h.IntelVADriverFix(),
			"Verify: ls /dev/dri/renderD* and run vainfo",
		}
	}
	switch h.Distro.Family {
	case DistroFamilyDebian:
		return []string{"Fix: install the VAAPI driver for your GPU (Intel: intel-media-va-driver-non-free; AMD: mesa-va-drivers)"}
	case DistroFamilyArch:
		return []string{"Fix: install intel-media-driver or mesa (AMD)"}
	default:
		return []string{"Fix: install the VAAPI driver for your GPU (Intel: intel-media-driver; AMD: mesa-va-drivers or mesa)"}
	}
}

// QSVRuntimeSkipMessage is the multi-line platform skip reason for missing QSV runtime.
func (h PackageHints) QSVRuntimeSkipMessage(plat Info) string {
	msg := "[driver] Intel oneVPL GPU runtime missing (libmfx-gen) — libvpl dispatcher alone cannot create QSV sessions"
	fix := h.QSVRuntimeFix()
	for _, line := range fix {
		if strings.HasPrefix(line, "Fix:") {
			msg += "\n" + line
			break
		}
	}
	if hint := plat.Details["qsv_runtime_hint"]; hint != "" {
		msg += "\nNote: " + hint
	}
	if plat.VAAPI {
		msg += "\nWorkaround: VAAPI encode/decode is working — use h264_vaapi/hevc_vaapi/av1_vaapi until QSV runtime is installed"
	}
	return msg
}

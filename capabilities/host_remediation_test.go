package capabilities

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestApplyHostRemediationFedoraAMD(t *testing.T) {
	host := DetectionHost{
		Presence: platform.GPUPresence{AMD: true},
		Distro:   platform.DistroContext{ID: "fedora", Family: platform.DistroFamilyRPM},
	}
	code, hint := ApplyHostRemediation(host, "vaapi", FFmpegBinaryInfo{Kind: FFmpegLinkDynamic},
		HWProbeVADriverMissing, HintForProbeCode(HWProbeVADriverMissing))
	if code != HWProbeGPUPresentDriverMissing {
		t.Fatalf("code=%s", code)
	}
	if !strings.Contains(hint, "mesa-va-drivers-freeworld") || !strings.Contains(hint, "RPM Fusion") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestApplyHostRemediationDebianIntel(t *testing.T) {
	host := DetectionHost{
		Presence: platform.GPUPresence{Intel: true},
		Distro:   platform.DistroContext{ID: "ubuntu", IDLike: "debian", Family: platform.DistroFamilyDebian},
	}
	code, hint := ApplyHostRemediation(host, "qsv", FFmpegBinaryInfo{Kind: FFmpegLinkDynamic},
		HWProbeLibvaMissing, "")
	if code != HWProbeGPUPresentDriverMissing {
		t.Fatalf("code=%s", hint)
	}
	if !strings.Contains(hint, "intel-media-va-driver-non-free") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestApplyHostRemediationDockerNVIDIAStatic(t *testing.T) {
	host := DetectionHost{
		Presence:  platform.GPUPresence{NVIDIA: true},
		Container: true,
		Distro:    platform.DistroContext{ID: "debian", Family: platform.DistroFamilyDebian},
	}
	code, hint := ApplyHostRemediation(host, "nvenc", FFmpegBinaryInfo{Kind: FFmpegLinkStatic},
		HWProbeStaticBinary, HintForProbeCode(HWProbeStaticBinary))
	if code != HWProbeGPUPresentDriverMissing {
		t.Fatalf("code=%s", code)
	}
	if !strings.Contains(strings.ToLower(hint), "nvidia container toolkit") || !strings.Contains(hint, "non-static") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestApplyHostRemediationSkipsWhenNoGPU(t *testing.T) {
	host := DetectionHost{Presence: platform.GPUPresence{}}
	code, hint := ApplyHostRemediation(host, "nvenc", FFmpegBinaryInfo{}, HWProbeNvidiaLibsMissing, "base")
	if code != HWProbeNvidiaLibsMissing || hint != "base" {
		t.Fatalf("code=%s hint=%q", code, hint)
	}
}

func TestHostRemediationArchAMD(t *testing.T) {
	host := DetectionHost{
		Presence: platform.GPUPresence{AMD: true},
		Distro:   platform.DistroContext{ID: "arch", Family: platform.DistroFamilyArch},
	}
	msg := HostRemediationMessage(host, "amd", "vaapi", FFmpegBinaryInfo{Kind: FFmpegLinkDynamic})
	if !strings.Contains(msg, "libva-mesa-driver") {
		t.Fatalf("msg=%q", msg)
	}
}

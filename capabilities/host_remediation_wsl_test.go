package capabilities

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func wslHost() DetectionHost {
	return DetectionHost{
		Presence: platform.GPUPresence{NVIDIA: true, WSLDXG: true},
		Distro:   platform.DistroContext{ID: "ubuntu", IDLike: "debian", Family: platform.DistroFamilyDebian},
		WSL:      true,
	}
}

// AV1 (and any missing NVENC codec unit) is a hardware generation limit, not a
// driver problem: it must not be promoted to gpu_present_driver_missing and it
// must not tell the operator to update the Windows driver.
func TestApplyHostRemediationAV1UnsupportedNotPromoted(t *testing.T) {
	host := wslHost()
	code, hint := ApplyHostRemediation(host, "nvenc", FFmpegBinaryInfo{Kind: FFmpegLinkDynamic},
		HWProbeNVENCUnsupported, HintForProbeCode(HWProbeNVENCUnsupported))
	if code != HWProbeNVENCUnsupported {
		t.Fatalf("code=%s, want %s (capability limit must not be promoted)", code, HWProbeNVENCUnsupported)
	}
	lower := strings.ToLower(hint)
	if strings.Contains(lower, "gpu detected") {
		t.Fatalf("hint=%q must not claim a GPU/driver fault", hint)
	}
	if strings.Contains(lower, "driver is current") || strings.Contains(lower, "wsl/lib is mounted") {
		t.Fatalf("hint=%q gives false WSL driver/mount remediation", hint)
	}
}

// A genuine NVIDIA userspace library miss on WSL still gets a WSL-aware hint,
// but the wording must not claim the Windows driver is out of date.
func TestApplyHostRemediationWSLNvidiaLibsMissing(t *testing.T) {
	host := wslHost()
	code, hint := ApplyHostRemediation(host, "nvenc", FFmpegBinaryInfo{Kind: FFmpegLinkDynamic},
		HWProbeNvidiaLibsMissing, HintForProbeCode(HWProbeNvidiaLibsMissing))
	if code != HWProbeGPUPresentDriverMissing {
		t.Fatalf("code=%s, want %s", code, HWProbeGPUPresentDriverMissing)
	}
	if !strings.Contains(hint, "/usr/lib/wsl/lib") {
		t.Fatalf("hint=%q should reference /usr/lib/wsl/lib", hint)
	}
	if strings.Contains(strings.ToLower(hint), "driver is current") {
		t.Fatalf("hint=%q contains the removed false driver-update text", hint)
	}
}

// WSL Vulkan has no NVIDIA ICD and no /dev/dri; the hint must say so and point
// at NVENC rather than a render node that cannot exist.
func TestWSLVulkanRemediationText(t *testing.T) {
	msg := wslRemediation(true, false, true, "vulkan", HWProbeVulkanNoVideoEncode)
	for _, want := range []string{"no NVIDIA Vulkan ICD", "NVENC"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("msg=%q missing %q", msg, want)
		}
	}
	for _, bad := range []string{"/dev/dri", "renderD128", "wsl --update", "install "} {
		if strings.Contains(msg, bad) {
			t.Fatalf("msg=%q must not suggest %q", msg, bad)
		}
	}
}

// The prerequisite layer must classify Vulkan-on-WSL as a missing video-encode
// capability, not a missing device.
func TestWSLVulkanPrereqClassifiesNoVideoEncode(t *testing.T) {
	deps := prereqDeps{
		Glob: func(string) ([]string, error) { return nil, nil },
		Open: func(string) (*os.File, error) { return nil, fs.ErrNotExist },
		Stat: func(name string) (os.FileInfo, error) {
			if name == "/usr/lib/wsl/lib/libvulkan.so.1" {
				return nil, nil
			}
			return nil, fs.ErrNotExist
		},
		ReadFile: func(string) ([]byte, error) { return nil, fs.ErrNotExist },
		LookPath: func(string) (string, error) { return "", fs.ErrNotExist },
		Ldconfig: func() ([]byte, error) {
			return []byte("libvulkan.so.1 => /usr/lib/x86_64-linux-gnu/libvulkan.so.1"), nil
		},
		Env: func(string) string { return "" },
	}
	plat := platform.Info{OS: "linux", WSL: true, NVIDIA: true, Vulkan: true, DRI: false}
	code, hint := checkVulkanPrereq(plat, deps)
	if code != HWProbeVulkanNoVideoEncode {
		t.Fatalf("code=%s hint=%q, want %s", code, hint, HWProbeVulkanNoVideoEncode)
	}
	if strings.Contains(hint, "/dev/dri/renderD128") || strings.Contains(hint, "wsl --update") {
		t.Fatalf("hint=%q suggests a render node / wsl update on WSL", hint)
	}
	if !strings.Contains(hint, "Vulkan ICD") {
		t.Fatalf("hint=%q should explain the missing NVIDIA Vulkan ICD", hint)
	}
}

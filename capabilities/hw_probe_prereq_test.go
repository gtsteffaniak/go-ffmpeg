package capabilities

import (
	"os"
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestPrereqRenderNodePermission(t *testing.T) {
	deps := prereqDeps{
		Glob: func(pattern string) ([]string, error) {
			if pattern == "/dev/dri/renderD*" {
				return []string{"/dev/dri/renderD128"}, nil
			}
			return nil, nil
		},
		Open: func(name string) (*os.File, error) {
			return nil, os.ErrPermission
		},
		Stat:     func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFile: func(string) ([]byte, error) { return []byte("render:x:993:\n"), nil },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Ldconfig: func() ([]byte, error) { return nil, os.ErrNotExist },
		Env:      func(string) string { return "" },
	}
	plat := platform.Info{OS: "linux", VAAPI: true, DRI: true}
	code, hint := checkVAStackPrereq("vaapi", plat, deps)
	if code != HWProbeDevicePermission {
		t.Fatalf("code=%s hint=%q", code, hint)
	}
	if !strings.Contains(hint, "render group gid 993") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestPrereqLibResolvableLDLibraryPath(t *testing.T) {
	deps := prereqDeps{
		Stat: func(name string) (os.FileInfo, error) {
			if name == "/opt/custom/libcuda.so.1" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		},
		Glob:     func(string) ([]string, error) { return nil, nil },
		Ldconfig: func() ([]byte, error) { return nil, os.ErrNotExist },
		Env: func(key string) string {
			if key == "LD_LIBRARY_PATH" {
				return "/opt/custom"
			}
			return ""
		},
	}
	if !libResolvable(deps, "libcuda.so.1") {
		t.Fatal("expected libcuda.so.1 via LD_LIBRARY_PATH")
	}
}

func TestPrereqStaticBinarySkipsInProbe(t *testing.T) {
	info := FFmpegBinaryInfo{Kind: FFmpegLinkStatic}
	if !gpuAccelBlockedByStaticBinary(info, "h264_qsv") {
		t.Fatal("qsv should be blocked")
	}
}

func TestPrereqNvidiaLibsViaLDLibraryPath(t *testing.T) {
	deps := prereqDeps{
		Stat: func(name string) (os.FileInfo, error) {
			switch name {
			case "/opt/nvidia/libcuda.so.1", "/opt/nvidia/libnvidia-encode.so.1":
				return nil, nil
			default:
				return nil, os.ErrNotExist
			}
		},
		Glob:     func(string) ([]string, error) { return nil, nil },
		Ldconfig: func() ([]byte, error) { return nil, os.ErrNotExist },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		Env: func(key string) string {
			if key == "LD_LIBRARY_PATH" {
				return "/opt/nvidia"
			}
			return ""
		},
	}
	plat := platform.Info{OS: "linux", NVIDIA: true}
	code, _ := checkNVENCPrereq(plat, deps)
	if code != HWProbeOK {
		t.Fatalf("code=%s want ok", code)
	}
}

func TestPrereqNvidiaLibsMissing(t *testing.T) {
	deps := defaultPrereqDeps()
	deps.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	deps.Ldconfig = func() ([]byte, error) { return []byte(""), nil }
	deps.Stat = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	plat := platform.Info{OS: "linux", NVIDIA: true}
	code, _ := checkNVENCPrereq(plat, deps)
	if code != HWProbeNvidiaLibsMissing {
		t.Fatalf("code=%s", code)
	}
}

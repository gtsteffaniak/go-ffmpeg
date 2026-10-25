package capabilities

import (
	"os"
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
}

func TestPrereqStaticBinarySkipsInProbe(t *testing.T) {
	info := FFmpegBinaryInfo{Kind: FFmpegLinkStatic}
	if !gpuAccelBlockedByStaticBinary(info, "h264_qsv") {
		t.Fatal("qsv should be blocked")
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

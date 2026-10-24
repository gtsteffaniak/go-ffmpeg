package capabilities

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// prereqDeps supports injecting fake filesystem state in unit tests.
type prereqDeps struct {
	Stat     func(name string) (os.FileInfo, error)
	Open     func(name string) (*os.File, error)
	Glob     func(pattern string) ([]string, error)
	ReadFile func(name string) ([]byte, error)
	LookPath func(file string) (string, error)
	Ldconfig func() ([]byte, error)
	Env func(key string) string
}

func defaultPrereqDeps() prereqDeps {
	return prereqDeps{
		Stat:     os.Stat,
		Open:     os.Open,
		Glob:     filepath.Glob,
		ReadFile: os.ReadFile,
		LookPath: exec.LookPath,
		Ldconfig: func() ([]byte, error) {
			return exec.Command("ldconfig", "-p").CombinedOutput()
		},
		Env: os.Getenv,
	}
}

// CheckHostPrerequisites validates host GPU stack before a functional encode probe.
// Windows and macOS skip Linux-specific checks (LoadLibrary / VideoToolbox work with dynamic ffmpeg).
func CheckHostPrerequisites(encoderKind string, plat platform.Info) (HWProbeCode, string) {
	return checkHostPrerequisites(encoderKind, plat, defaultPrereqDeps())
}

func checkHostPrerequisites(encoderKind string, plat platform.Info, deps prereqDeps) (HWProbeCode, string) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return HWProbeOK, ""
	}
	switch encoderKind {
	case "software", "native", "videotoolbox":
		return HWProbeOK, ""
	case "v4l2m2m":
		return checkV4L2Prereq(deps)
	case "vaapi", "qsv":
		return checkVAStackPrereq(encoderKind, plat, deps)
	case "nvenc":
		return checkNVENCPrereq(plat, deps)
	case "amf":
		if plat.OS == "windows" {
			return HWProbeOK, ""
		}
		return HWProbeNoDevice, HintForProbeCode(HWProbeNoDevice)
	default:
		return HWProbeOK, ""
	}
}

func checkV4L2Prereq(deps prereqDeps) (HWProbeCode, string) {
	matches, _ := deps.Glob("/dev/video*")
	for _, path := range matches {
		if strings.Contains(path, "video") {
			if f, err := deps.Open(path); err == nil {
				f.Close()
				return HWProbeOK, ""
			}
		}
	}
	return HWProbeNoDevice, HintForProbeCode(HWProbeNoDevice)
}

func checkVAStackPrereq(encoderKind string, plat platform.Info, deps prereqDeps) (HWProbeCode, string) {
	if plat.WSL {
		if _, err := deps.Stat("/dev/dxg"); err != nil {
			return HWProbeNoDevice, "WSL GPU partitioning requires /dev/dxg; enable WSL2 GPU support in Windows"
		}
		if _, err := deps.Stat("/usr/lib/wsl/lib"); err != nil {
			return HWProbeNvidiaLibsMissing, "WSL userspace libraries missing under /usr/lib/wsl/lib"
		}
		if !vaDriverResolvable(deps, "d3d12_drv_video.so") && !plat.DRI {
			// Mesa d3d12 VA driver is the usual WSL path when DRI is absent.
			if !vaDriverResolvable(deps, "") {
				return HWProbeVADriverMissing, HintForProbeCode(HWProbeVADriverMissing)
			}
		}
	}
	if !renderNodeOpenable(deps) {
		if plat.WSL && !plat.DRI {
			// NVENC-only WSL without DRI is valid; QSV/VAAPI need the d3d12 stack above.
			return HWProbeOK, ""
		}
		code := HWProbeNoDevice
		hint := HintForProbeCode(code)
		if gid, ok := renderGroupGID(deps); ok {
			hint += fmt.Sprintf(" (render group gid %d)", gid)
		}
		return code, hint
	}
	if _, err := deps.LookPath("libva.so.2"); err != nil {
		if !libResolvable(deps, "libva.so.2") {
			return HWProbeLibvaMissing, HintForProbeCode(HWProbeLibvaMissing)
		}
	}
	return HWProbeOK, ""
}

func checkNVENCPrereq(plat platform.Info, deps prereqDeps) (HWProbeCode, string) {
	if !plat.NVIDIA {
		return HWProbeNoDevice, HintForProbeCode(HWProbeNoDevice)
	}
	if plat.WSL {
		if _, err := deps.Stat("/dev/dxg"); err != nil {
			return HWProbeNoDevice, "WSL NVIDIA encode requires /dev/dxg (wsl --update, GPU-PV enabled)"
		}
		if _, err := deps.Stat("/usr/lib/wsl/lib"); err != nil {
			return HWProbeNvidiaLibsMissing, HintForProbeCode(HWProbeNvidiaLibsMissing)
		}
	}
	if !libResolvable(deps, "libcuda.so.1") || !libResolvable(deps, "libnvidia-encode.so.1") {
		return HWProbeNvidiaLibsMissing, HintForProbeCode(HWProbeNvidiaLibsMissing)
	}
	return HWProbeOK, ""
}

func renderNodeOpenable(deps prereqDeps) bool {
	nodes, _ := deps.Glob("/dev/dri/renderD*")
	for _, node := range nodes {
		f, err := deps.Open(node)
		if err != nil {
			continue
		}
		f.Close()
		return true
	}
	return false
}

func renderGroupGID(deps prereqDeps) (int, bool) {
	data, err := deps.ReadFile("/etc/group")
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "render:") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}
		var gid int
		if _, err := fmt.Sscanf(parts[2], "%d", &gid); err == nil {
			return gid, true
		}
	}
	return 0, false
}

func libResolvable(deps prereqDeps, soname string) bool {
	if soname == "" {
		return false
	}
	if path, err := deps.LookPath(soname); err == nil && path != "" {
		return true
	}
	out, err := deps.Ldconfig()
	if err == nil && strings.Contains(string(out), soname) {
		return true
	}
	search := []string{
		"/usr/lib/x86_64-linux-gnu/" + soname,
		"/usr/lib/wsl/lib/" + soname,
		"/usr/lib64/" + soname,
		"/usr/lib/" + soname,
	}
	for _, p := range search {
		if _, err := deps.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func vaDriverResolvable(deps prereqDeps, driverName string) bool {
	want := driverName
	if want != "" && !strings.HasSuffix(want, "_drv_video.so") {
		want = want + "_drv_video.so"
	}
	var paths []string
	if v := deps.Env("LIBVA_DRIVERS_PATH"); v != "" {
		for _, dir := range strings.Split(v, ":") {
			if want != "" {
				paths = append(paths, filepath.Join(dir, want))
			} else {
				matches, _ := deps.Glob(filepath.Join(dir, "*_drv_video.so"))
				paths = append(paths, matches...)
			}
		}
	}
	for _, g := range []string{
		"/usr/lib/x86_64-linux-gnu/dri/*_drv_video.so",
		"/usr/lib64/dri/*_drv_video.so",
		"/usr/lib/dri/*_drv_video.so",
	} {
		matches, _ := deps.Glob(g)
		paths = append(paths, matches...)
	}
	for _, p := range paths {
		if want != "" && filepath.Base(p) != want {
			continue
		}
		if _, err := deps.Stat(p); err == nil {
			return true
		}
	}
	return false
}

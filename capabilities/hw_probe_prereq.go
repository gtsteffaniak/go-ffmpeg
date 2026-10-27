package capabilities

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	Env      func(key string) string
}

func defaultPrereqDeps() prereqDeps {
	return prereqDeps{
		Stat:     os.Stat,
		Open:     os.Open,
		Glob:     filepath.Glob,
		ReadFile: os.ReadFile,
		LookPath: exec.LookPath,
		Ldconfig: runLdconfigP,
		Env:      os.Getenv,
	}
}

// CheckHostPrerequisites validates the host GPU stack before a functional encode probe.
// It runs once per encoder during detection and does not retry. On Windows and macOS
// Linux-specific checks are skipped (LoadLibrary / VideoToolbox). Returns [HWProbeOK]
// when prerequisites pass, otherwise a code and hint (see [HintForProbeCode]).
// Subprocess helpers (ldconfig) use bounded timeouts.
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
	case "vulkan":
		return checkVulkanPrereq(plat, deps)
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
	exists, openable := renderNodeStatus(deps)
	if !openable {
		if plat.WSL && !plat.DRI {
			// NVENC-only WSL without DRI is valid; QSV/VAAPI need the d3d12 stack above.
			return HWProbeOK, ""
		}
		if exists {
			return renderNodePermissionResult(deps)
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

func checkVulkanPrereq(plat platform.Info, deps prereqDeps) (HWProbeCode, string) {
	if plat.OS == "darwin" {
		return HWProbeSkipped, "Vulkan video encode is not supported on macOS"
	}
	if plat.Details != nil && plat.Details["pi_model"] == string(platform.PiLegacy) {
		return HWProbeNoDevice, "Vulkan video encode is not supported on this Raspberry Pi model"
	}
	if !vulkanLoaderResolvable(deps) {
		return HWProbeVulkanLoaderMissing, HintForProbeCode(HWProbeVulkanLoaderMissing)
	}
	if plat.OS == "windows" {
		return HWProbeOK, ""
	}
	exists, openable := renderNodeStatus(deps)
	if !openable {
		if exists {
			return renderNodePermissionResult(deps)
		}
		// WSL2 has no /dev/dri render node by design (GPU paravirtualization
		// uses /dev/dxg) and no NVIDIA Vulkan ICD, so Vulkan video encode is
		// simply unavailable there. Report the capability limit rather than
		// asking the operator to mount a render node that cannot exist, and do
		// not tell them to update a driver that is already working.
		if plat.WSL {
			return HWProbeVulkanNoVideoEncode, wslPlatformRemediation(plat, "vulkan", HWProbeVulkanNoVideoEncode)
		}
		code := HWProbeVulkanNoDevice
		hint := HintForProbeCode(code)
		if gid, ok := renderGroupGID(deps); ok {
			hint += fmt.Sprintf(" (render group gid %d)", gid)
		}
		return code, hint
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

// renderNodeStatus reports whether any /dev/dri/renderD* node exists and is openable.
func renderNodeStatus(deps prereqDeps) (exists, openable bool) {
	nodes, _ := deps.Glob("/dev/dri/renderD*")
	for _, node := range nodes {
		exists = true
		f, err := deps.Open(node)
		if err != nil {
			continue
		}
		f.Close()
		return true, true
	}
	return exists, false
}

// renderNodePermissionResult builds [HWProbeDevicePermission] with optional render GID.
func renderNodePermissionResult(deps prereqDeps) (HWProbeCode, string) {
	hint := HintForProbeCode(HWProbeDevicePermission)
	if gid, ok := renderGroupGID(deps); ok {
		hint += fmt.Sprintf(" (render group gid %d)", gid)
	}
	return HWProbeDevicePermission, hint
}

// runLdconfigP runs ldconfig -p with a 5s timeout (tries sbin paths).
func runLdconfigP() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, bin := range []string{"ldconfig", "/sbin/ldconfig", "/usr/sbin/ldconfig"} {
		out, err := exec.CommandContext(ctx, bin, "-p").CombinedOutput()
		if err == nil {
			return out, nil
		}
	}
	return nil, fmt.Errorf("ldconfig unavailable")
}

func vulkanLoaderResolvable(deps prereqDeps) bool {
	if runtime.GOOS == "windows" {
		return libResolvable(deps, "vulkan-1.dll")
	}
	return libResolvable(deps, "libvulkan.so.1")
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

// libResolvable checks ldconfig, multiarch paths, and LD_LIBRARY_PATH for soname.
func libResolvable(deps prereqDeps, soname string) bool {
	if soname == "" {
		return false
	}
	out, err := deps.Ldconfig()
	if err == nil && strings.Contains(string(out), soname) {
		return true
	}
	if matches, _ := deps.Glob("/usr/lib/*-linux-gnu/" + soname); len(matches) > 0 {
		return true
	}
	search := []string{
		"/usr/lib/x86_64-linux-gnu/" + soname,
		"/usr/lib/aarch64-linux-gnu/" + soname,
		"/usr/lib/wsl/lib/" + soname,
		"/usr/lib64/" + soname,
		"/usr/lib/" + soname,
	}
	if runtime.GOOS == "windows" {
		search = append(search,
			"C:\\Windows\\System32\\"+soname,
			"C:\\Windows\\SysWOW64\\"+soname,
		)
	}
	for _, p := range search {
		if _, err := deps.Stat(p); err == nil {
			return true
		}
	}
	if v := deps.Env("LD_LIBRARY_PATH"); v != "" {
		for _, dir := range strings.Split(v, ":") {
			if dir == "" {
				continue
			}
			if _, err := deps.Stat(filepath.Join(dir, soname)); err == nil {
				return true
			}
		}
	}
	return false
}

// vaDriverResolvable checks LIBVA_DRIVERS_PATH and standard dri directories.
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

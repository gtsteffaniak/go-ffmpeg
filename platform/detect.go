package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Detect probes host platform for GPU/driver availability.
func Detect() Info {
	info := Info{
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Details: map[string]string{},
	}
	info.WSL = isWSLEnvironment()
	info.NVIDIA = nvidiaAvailable()
	info.Intel = intelGPUPresent()
	info.AMD = amdGPUPresent()
	info.DRI = driAvailable()
	if info.DRI {
		if nodes, err := filepath.Glob("/dev/dri/renderD*"); err == nil && len(nodes) > 0 {
			info.Details["render_device"] = nodes[0]
		}
	}
	info.VAAPI, info.Details["vaapi_driver"] = vaapiAvailableDetailed()
	if info.Intel && !info.VAAPI {
		info.Details["intel_va_hint"] = "install intel-media-va-driver or intel-media-va-driver-non-free for Quick Sync / VAAPI on Intel GPUs"
	}
	info.QSV = qsvAvailable(info.WSL, info.Intel)
	if runtime.GOOS == "linux" && info.Intel {
		info.QSVRuntime, info.Details["qsv_runtime_lib"] = qsvRuntimeAvailable()
		if dispatcher, ok := oneVPLDispatcherAvailable(); ok {
			info.Details["vpl_dispatcher"] = dispatcher
		}
		if info.QSV && !info.QSVRuntime {
			info.Details["qsv_runtime_hint"] = "install libmfx-gen1.2 (oneVPL GPU runtime); libvpl2 alone is not sufficient for QSV"
		}
	} else if runtime.GOOS == "windows" && info.Intel {
		// Windows bundles runtime with Intel graphics driver; validated at encode time.
		info.QSVRuntime = true
	}
	info.D3D12 = d3d12Available(info.WSL)
	info.Details["gpu"] = detectGPUDescription()
	return info
}

func nvidiaAvailable() bool {
	switch runtime.GOOS {
	case "windows":
		for _, path := range []string{
			`C:\Windows\System32\nvapi64.dll`,
			`C:\Windows\System32\nvcuda.dll`,
			`C:\Windows\System32\nvml.dll`,
		} {
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
		cmd := exec.Command("reg", "query", `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`, "/s", "/f", "NVIDIA")
		cmd.Stdout = nil
		cmd.Stderr = nil
		return cmd.Run() == nil
	case "linux":
		for _, device := range []string{"/dev/nvidia0", "/dev/nvidiactl", "/dev/nvidia-uvm"} {
			if _, err := os.Stat(device); err == nil {
				return true
			}
		}
		if out, err := exec.Command("lsmod").Output(); err == nil && strings.Contains(string(out), "nvidia") {
			return true
		}
		cmd := exec.Command("nvidia-smi", "-L")
		cmd.Stdout = nil
		cmd.Stderr = nil
		return cmd.Run() == nil
	}
	return false
}

func amdGPUPresent() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if drmVendorPresent("0x1002") {
		return true
	}
	if commandAvailable("lspci") {
		out, err := exec.Command("lspci", "-nn").CombinedOutput()
		if err == nil {
			s := strings.ToLower(string(out))
			if strings.Contains(s, "[1002:") || strings.Contains(s, "amd/ati") {
				return true
			}
		}
	}
	return false
}

func intelGPUPresent() bool {
	if runtime.GOOS == "windows" {
		return true // validated at encode time
	}
	if drmVendorPresent("0x8086") {
		return true
	}
	if intelFromCPUInfo() {
		return true
	}
	return intelFromPCI()
}

func drmVendorPresent(vendorID string) bool {
	matches, _ := filepath.Glob("/sys/class/drm/card*/device/vendor")
	for _, vendorFile := range matches {
		if data, err := os.ReadFile(vendorFile); err == nil {
			if strings.EqualFold(strings.TrimSpace(string(data)), vendorID) {
				return true
			}
		}
	}
	return false
}

func intelFromCPUInfo() bool {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return false
	}
	cpuInfo := strings.ToLower(string(data))
	if !strings.Contains(cpuInfo, "intel") {
		return false
	}
	markers := []string{"graphics", "hd graphics", "uhd graphics", "iris", "arc", "lunar", "meteor", "arrow", "raptor"}
	for _, m := range markers {
		if strings.Contains(cpuInfo, m) {
			return true
		}
	}
	return false
}

func intelFromPCI() bool {
	if !commandAvailable("lspci") {
		return false
	}
	out, err := exec.Command("lspci", "-nn").CombinedOutput()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	if !strings.Contains(s, "intel") {
		return false
	}
	markers := []string{
		"vga", "display", "xe", "arc", "iris", "uhd", "hd graphics",
		"lunar", "arrow", "meteor", "raptor", "alder", "rocket", "dg",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	// Intel vendor id on display class devices
	return strings.Contains(s, "[8086:")
}

func driAvailable() bool {
	if runtime.GOOS == "linux" {
		matches, _ := filepath.Glob("/dev/dri/renderD*")
		return len(matches) > 0
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return false
}

func qsvAvailable(wsl, intel bool) bool {
	if wsl {
		return wslIntelGPU() || intel
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		return intel
	}
	return false
}

// qsvRuntimeAvailable reports whether the oneVPL GPU runtime (libmfx-gen) is installed.
// libvpl2 is only the dispatcher; FFmpeg QSV needs the hardware runtime on Linux.
func qsvRuntimeAvailable() (bool, string) {
	globs := []string{
		"/usr/lib/x86_64-linux-gnu/libmfx-gen.so*",
		"/usr/lib64/libmfx-gen.so*",
		"/usr/lib/libmfx-gen.so*",
	}
	for _, pattern := range globs {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return true, matches[0]
		}
	}
	// Legacy Media SDK runtime (older distros / GPUs)
	for _, pattern := range []string{
		"/usr/lib/x86_64-linux-gnu/libmfxhw64.so*",
		"/usr/lib64/libmfxhw64.so*",
	} {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return true, matches[0]
		}
	}
	return false, ""
}

func oneVPLDispatcherAvailable() (string, bool) {
	paths := []string{
		"/usr/lib/x86_64-linux-gnu/libvpl.so.2",
		"/usr/lib64/libvpl.so.2",
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

func wslIntelGPU() bool {
	if commandAvailable("lspci") {
		out, err := exec.Command("lspci").CombinedOutput()
		if err == nil && strings.Contains(string(out), "Microsoft Corporation Basic Render Driver") {
			return true
		}
	}
	if matches, _ := filepath.Glob("/usr/lib/wsl/lib*"); len(matches) > 0 {
		return true
	}
	if _, err := os.Stat("/dev/dxg"); err == nil {
		return true
	}
	return false
}

func vaapiAvailableDetailed() (bool, string) {
	globs := []string{
		"/usr/lib/x86_64-linux-gnu/dri/*_drv_video.so",
		"/usr/lib64/dri/*_drv_video.so",
		"/usr/lib/dri/*_drv_video.so",
	}
	var all []string
	for _, pattern := range globs {
		matches, _ := filepath.Glob(pattern)
		all = append(all, matches...)
	}
	if len(all) == 0 {
		return false, ""
	}

	intel := intelGPUPresent()
	amd := amdGPUPresent()

	// Intel Quick Sync / VAAPI on Linux needs the Intel media driver (iHD for Gen8+ / Lunar Lake).
	if intel {
		for _, name := range []string{"iHD_drv_video.so", "i965_drv_video.so", "intel_drv_video.so"} {
			for _, m := range all {
				if filepath.Base(m) == name {
					return true, m
				}
			}
		}
		// Intel GPU but no Intel VA driver — do not treat unrelated Mesa drivers as VAAPI-ready.
		return false, ""
	}

	if amd {
		for _, name := range []string{"radeonsi_drv_video.so", "r600_drv_video.so"} {
			for _, m := range all {
				if filepath.Base(m) == name {
					return true, m
				}
			}
		}
	}

	for _, name := range []string{"nouveau_drv_video.so", "d3d12_drv_video.so", "virtio_gpu_drv_video.so"} {
		for _, m := range all {
			if filepath.Base(m) == name {
				return true, m
			}
		}
	}
	return true, all[0]
}

func d3d12Available(wsl bool) bool {
	if runtime.GOOS != "linux" || !wsl {
		return false
	}
	if _, err := os.Stat("/dev/dxg"); err != nil {
		return false
	}
	if !wsl2Kernel() {
		return false
	}
	ok, _ := vaapiAvailableDetailed()
	return ok
}

func detectGPUDescription() string {
	if !commandAvailable("lspci") {
		return ""
	}
	out, err := exec.Command("lspci", "-nn").CombinedOutput()
	if err != nil {
		return ""
	}
	var gpus []string
	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "vga") || strings.Contains(lower, "3d") || strings.Contains(lower, "display") {
			gpus = append(gpus, strings.TrimSpace(line))
		}
	}
	return strings.Join(gpus, "; ")
}

func isWSLEnvironment() bool {
	indicators := []string{"/proc/version", "/proc/sys/kernel/osrelease", "/proc/sys/fs/binfmt_misc/WSLInterop"}
	for _, indicator := range indicators {
		if content, err := os.ReadFile(indicator); err == nil {
			s := string(content)
			if strings.Contains(strings.ToLower(s), "microsoft") || strings.Contains(s, "WSL") {
				return true
			}
		}
	}
	return false
}

func wsl2Kernel() bool {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "microsoft")
}

func commandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// RenderDevice returns the preferred DRM render node for hw tests.
func RenderDevice(details map[string]string) string {
	if details != nil {
		if dev := details["render_device"]; dev != "" {
			return dev
		}
	}
	matches, _ := filepath.Glob("/dev/dri/renderD*")
	if len(matches) > 0 {
		return matches[0]
	}
	return "/dev/dri/renderD128"
}

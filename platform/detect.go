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
	if runtime.GOOS == "linux" {
		info.Distro = DetectDistro()
	}
	info.WSL = isWSLEnvironment()
	if info.WSL {
		info.WSLGPUPartitioning = wslGPUPartitioning()
		if info.WSLGPUPartitioning {
			info.Details["wsl_gpu_partitioning"] = "active"
		}
	}
	info.NVIDIA = nvidiaAvailable()
	info.Intel = intelGPUPresent()
	info.AMD = amdGPUPresent()
	info.DRI = driAvailable()
	if info.DRI {
		if gpus, err := listDRMGPUs(); err == nil && len(gpus) > 0 {
			info.Details["render_device"] = DefaultRenderNodeForGPUs(gpus)
		} else if nodes, err := filepath.Glob("/dev/dri/renderD*"); err == nil && len(nodes) > 0 {
			info.Details["render_device"] = nodes[0]
		}
	}
	info.VAAPI, info.Details["vaapi_driver"] = vaapiAvailableDetailed()
	if info.Intel && !info.VAAPI {
		hints := NewPackageHints(info.Distro)
		info.Details["intel_va_hint"] = "install Intel VAAPI driver (" + strings.Join(hints.Distro.IntelVADriverPackages(), " or ") + ")"
	}
	info.QSV = qsvAvailable(info.WSL, info.Intel)
	if runtime.GOOS == "linux" && info.Intel {
		info.QSVRuntime, info.Details["qsv_runtime_lib"] = qsvRuntimeAvailable()
		if dispatcher, ok := oneVPLDispatcherAvailable(); ok {
			info.Details["vpl_dispatcher"] = dispatcher
		}
		if info.QSV && !info.QSVRuntime {
			info.Details["qsv_runtime_hint"] = info.Distro.QSVRuntimeHint()
		}
	} else if runtime.GOOS == "windows" && info.Intel {
		// Windows bundles runtime with Intel graphics driver; validated at encode time.
		info.QSVRuntime = true
	}
	info.D3D12 = d3d12Available(info.WSL)
	info.VideoToolbox = runtime.GOOS == "darwin"
	info.Details["gpu"] = detectGPUDescription()
	if pi := DetectPiModel(); pi != PiUnknown {
		info.Details["pi_model"] = string(pi)
		prof := PiEncodeProfileForModel(pi)
		info.Details["pi_h264_summary"] = prof.H264Summary
		if prof.InstallHint != "" {
			info.Details["pi_install_hint"] = prof.InstallHint
		}
		if v4l := ListV4L2M2MDevices(); len(v4l) > 0 {
			var names []string
			for _, d := range v4l {
				names = append(names, d.Path+" ("+d.Name+")")
			}
			info.Details["v4l2_m2m"] = strings.Join(names, "; ")
		}
	}
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
		return runSilentTimeout("reg", "query", `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`, "/s", "/f", "NVIDIA") == nil
	case "linux":
		for _, device := range []string{"/dev/nvidia0", "/dev/nvidiactl", "/dev/nvidia-uvm"} {
			if _, err := os.Stat(device); err == nil {
				return true
			}
		}
		if out, err := combinedOutputTimeout("lsmod"); err == nil && strings.Contains(string(out), "nvidia") {
			return true
		}
		return runSilentTimeout("nvidia-smi", "-L") == nil
	}
	return false
}

func intelGPUPresent() bool {
	if runtime.GOOS == "windows" {
		gpus := listWindowsGPUs()
		if len(gpus) == 0 {
			return true // enumeration unavailable; validated at encode time
		}
		for _, g := range gpus {
			if g.Vendor == "intel" {
				return true
			}
		}
		return false
	}
	if isWSLEnvironment() {
		// WSL guests cannot see PCI/DRM adapters the paravirt channel did not
		// register; the host's own adapter list is the source of truth.
		return wslVendorPresent("intel", func() bool {
			return intelFromCPUInfo() || intelFromPCI()
		})
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
	return intelGPUPresentInCPUInfo(string(data))
}

func intelGPUPresentInCPUInfo(cpuInfo string) bool {
	for _, line := range strings.Split(cpuInfo, "\n") {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "model name") {
			continue
		}
		if !strings.Contains(strings.ToLower(line), "intel") {
			continue
		}
		if lineHasIntelGPUMarker(line) {
			return true
		}
	}
	return false
}

func intelFromPCI() bool {
	if !commandAvailable("lspci") {
		return false
	}
	out, err := combinedOutputTimeout("lspci", "-nn")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "intel") {
			continue
		}
		if !isDisplayPCILine(line) && !strings.Contains(lower, "[8086:") {
			continue
		}
		if lineHasIntelGPUMarker(line) {
			return true
		}
		if strings.Contains(lower, "[8086:") && isDisplayPCILine(line) {
			return true
		}
	}
	return false
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
		// GPU paravirtualization alone says nothing about the vendor: an
		// NVIDIA- or AMD-only WSL host must not report Quick Sync.
		return intel
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		return intel
	}
	return false
}

// wslGPUPartitioning detects WSL2 GPU paravirtualization (Intel/AMD/NVIDIA via /dev/dxg).
func wslGPUPartitioning() bool {
	if _, err := os.Stat("/dev/dxg"); err == nil {
		return true
	}
	if matches, _ := filepath.Glob("/usr/lib/wsl/lib*"); len(matches) > 0 {
		return true
	}
	for _, path := range []string{"/mnt/wslg", "/mnt/wslg/versions.txt", "/usr/lib/wsl/lib"} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	if os.Getenv("GALLIUM_DRIVER") == "d3d12" {
		return true
	}
	if commandAvailable("lspci") {
		out, err := combinedOutputTimeout("lspci")
		if err == nil {
			s := string(out)
			if strings.Contains(s, "Microsoft Corporation Basic Render Driver") {
				return true
			}
		}
		out, err = combinedOutputTimeout("lspci", "-nn")
		if err == nil {
			s := strings.ToLower(string(out))
			if strings.Contains(s, "microsoft") && strings.Contains(s, "basic render") {
				return true
			}
		}
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

	// On WSL every VA driver (iHD, radeonsi, d3d12) still opens a DRM render
	// node underneath; without /dev/dri the driver stack is unusable.
	if isWSLEnvironment() && !driAvailable() {
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

	// Remaining drivers each serve a specific device class. Distros ship the
	// nouveau/d3d12 blobs with Mesa on every machine, so the file existing says
	// nothing — credit it only when the matching kernel driver/device is bound.
	for _, m := range all {
		switch filepath.Base(m) {
		case "nouveau_drv_video.so":
			if drmDriverBound("nouveau") {
				return true, m
			}
		case "d3d12_drv_video.so":
			if isWSLEnvironment() && wslGPUPartitioning() {
				return true, m
			}
		case "virtio_gpu_drv_video.so":
			if drmVendorPresent("0x1af4") || drmDriverBound("virtio_gpu") {
				return true, m
			}
		default:
			// Unknown VA driver (e.g. the nvidia-vaapi-driver shim): only credit
			// it when a DRM render node exists for it to open.
			if driAvailable() {
				return true, m
			}
		}
	}
	return false, ""
}

// drmDriverBound reports whether a DRM card's device is bound to the named kernel driver.
func drmDriverBound(name string) bool {
	matches, _ := filepath.Glob("/sys/class/drm/card*/device/driver")
	for _, link := range matches {
		if target, err := filepath.EvalSymlinks(link); err == nil && filepath.Base(target) == name {
			return true
		}
	}
	return false
}

func d3d12Available(wsl bool) bool {
	if runtime.GOOS != "linux" || !wsl {
		return false
	}
	if !wslGPUPartitioning() {
		return false
	}
	if !wsl2Kernel() {
		return false
	}
	ok, driver := vaapiAvailableDetailed()
	if !ok {
		return false
	}
	if strings.Contains(filepath.Base(driver), "d3d12") {
		return true
	}
	// DXGK + WSL2 + any working VAAPI driver (often d3d12 backend in WSL).
	return true
}

func detectGPUDescription() string {
	var parts []string
	if runtime.GOOS == "windows" {
		for _, g := range listWindowsGPUs() {
			parts = append(parts, g.Name)
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	if isWSLEnvironment() {
		for _, g := range listWSLGPUs() {
			parts = append(parts, g.Name)
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	if name := NVIDIAGPUName(); name != "" {
		parts = append(parts, name)
	}
	if !commandAvailable("lspci") {
		return strings.Join(parts, "; ")
	}
	out, err := combinedOutputTimeout("lspci", "-nn")
	if err != nil {
		return strings.Join(parts, "; ")
	}
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		lines = append(lines, line)
	}
	gpus := FilterDisplayControllerLines(lines)
	if len(gpus) > 0 {
		parts = append(parts, strings.Join(gpus, "; "))
	}
	return strings.Join(parts, "; ")
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

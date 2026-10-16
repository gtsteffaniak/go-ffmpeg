package platform

import (
	"bufio"
	"os"
	"strings"
)

// DistroFamily groups Linux distributions by package manager for install hints.
type DistroFamily string

const (
	DistroFamilyUnknown DistroFamily = ""
	DistroFamilyDebian  DistroFamily = "debian" // apt: Debian, Ubuntu, derivatives
	DistroFamilyArch    DistroFamily = "arch"   // pacman
	DistroFamilyRPM     DistroFamily = "rpm"    // dnf/yum: Fedora, RHEL, Alma, etc.
	DistroFamilySUSE    DistroFamily = "suse"   // zypper: openSUSE, SLE
	DistroFamilyAlpine  DistroFamily = "alpine" // apk
)

// DistroContext records the detected host distribution for package hints.
type DistroContext struct {
	ID     string       `json:"id,omitempty"`
	IDLike string       `json:"idLike,omitempty"`
	Name   string       `json:"name,omitempty"`
	Family DistroFamily `json:"family,omitempty"`
}

// DetectDistro reads /etc/os-release when present.
func DetectDistro() DistroContext {
	return ParseOSRelease("/etc/os-release")
}

// ParseOSRelease parses a freedesktop os-release file (used in tests with fixtures).
func ParseOSRelease(path string) DistroContext {
	f, err := os.Open(path)
	if err != nil {
		return DistroContext{Family: DistroFamilyUnknown}
	}
	defer f.Close()

	vals := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = strings.Trim(val, `"`)
		vals[strings.ToLower(strings.TrimSpace(key))] = val
	}

	id := strings.ToLower(vals["id"])
	idLike := strings.ToLower(vals["id_like"])
	name := vals["name"]

	return DistroContext{
		ID:     id,
		IDLike: idLike,
		Name:   name,
		Family: classifyDistroFamily(id, idLike),
	}
}

func classifyDistroFamily(id, idLike string) DistroFamily {
	check := id + " " + idLike
	switch {
	case id == "arch" || strings.Contains(check, "arch"):
		return DistroFamilyArch
	case id == "alpine" || strings.Contains(check, "alpine"):
		return DistroFamilyAlpine
	case strings.Contains(check, "opensuse") || strings.Contains(check, "suse"):
		return DistroFamilySUSE
	case id == "fedora" || strings.Contains(check, "fedora") ||
		id == "rhel" || strings.Contains(check, "rhel") ||
		id == "centos" || strings.Contains(check, "centos") ||
		id == "almalinux" || strings.Contains(check, "almalinux") ||
		id == "rocky" || strings.Contains(check, "rocky") ||
		strings.Contains(check, "nobara"):
		return DistroFamilyRPM
	case id == "debian" || strings.Contains(check, "debian") ||
		id == "ubuntu" || strings.Contains(check, "ubuntu") ||
		strings.Contains(check, "linuxmint") ||
		strings.Contains(check, "pop"):
		return DistroFamilyDebian
	default:
		return DistroFamilyUnknown
	}
}

// InstallCommand returns a shell command to install the given package names.
func (d DistroContext) InstallCommand(packages ...string) string {
	if len(packages) == 0 {
		return ""
	}
	pkgs := strings.Join(packages, " ")
	switch d.Family {
	case DistroFamilyDebian:
		return "sudo apt install " + pkgs
	case DistroFamilyArch:
		return "sudo pacman -S " + pkgs
	case DistroFamilyRPM:
		return "sudo dnf install " + pkgs
	case DistroFamilySUSE:
		return "sudo zypper install " + pkgs
	case DistroFamilyAlpine:
		return "sudo apk add " + pkgs
	default:
		return "install " + pkgs + " using your distro package manager"
	}
}

// QSVRuntimePackages returns verified distro package names for the Intel oneVPL GPU runtime.
func (d DistroContext) QSVRuntimePackages() []string {
	switch d.Family {
	case DistroFamilyArch:
		return []string{"vpl-gpu-rt", "libvpl", "intel-media-driver"}
	case DistroFamilyDebian:
		return []string{"libmfx-gen1.2", "libvpl2", "intel-media-va-driver-non-free"}
	case DistroFamilyRPM:
		return []string{"intel-vpl-gpu-rt", "libvpl", "intel-media-driver"}
	case DistroFamilySUSE:
		return []string{"libmfx-gen", "libvpl2", "intel-media-driver"}
	case DistroFamilyAlpine:
		// oneVPL GPU runtime packaging varies; point at libvpl meta when unsure.
		return []string{"libvpl", "intel-media-driver"}
	default:
		return nil
	}
}

// IntelVADriverPackages returns packages for Intel VAAPI on Linux.
func (d DistroContext) IntelVADriverPackages() []string {
	switch d.Family {
	case DistroFamilyArch:
		return []string{"intel-media-driver"}
	case DistroFamilyDebian:
		return []string{"intel-media-va-driver-non-free"}
	case DistroFamilyRPM:
		return []string{"intel-media-driver"}
	case DistroFamilySUSE:
		return []string{"intel-media-driver"}
	case DistroFamilyAlpine:
		return []string{"intel-media-driver"}
	default:
		return []string{"intel-media-va-driver or intel-media-driver (distro-specific)"}
	}
}

// QSVRuntimeHint is a short note for platform.Details and logs.
func (d DistroContext) QSVRuntimeHint() string {
	pkgs := d.QSVRuntimePackages()
	if len(pkgs) == 0 {
		return "install the Intel oneVPL GPU runtime (provides libmfx-gen); libvpl alone is not sufficient for QSV"
	}
	return "install " + pkgs[0] + " (oneVPL GPU runtime); libvpl dispatcher alone is not sufficient for QSV"
}

// QSVStackDescription explains the required QSV stack for report hints.
func (d DistroContext) QSVStackDescription() string {
	pkgs := d.QSVRuntimePackages()
	if len(pkgs) == 0 {
		return "Required stack: Intel VAAPI driver + libvpl dispatcher + oneVPL GPU runtime (libmfx-gen)"
	}
	return "Required stack: " + strings.Join(pkgs, " + ")
}

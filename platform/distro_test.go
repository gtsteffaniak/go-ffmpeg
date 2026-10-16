package platform_test

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestParseOSReleaseArch(t *testing.T) {
	path := filepath.Join("testdata", "os-release", "archlinux")
	d := platform.ParseOSRelease(path)
	if d.ID != "arch" {
		t.Fatalf("id = %q", d.ID)
	}
	if d.Family != platform.DistroFamilyArch {
		t.Fatalf("family = %q", d.Family)
	}
	pkgs := d.QSVRuntimePackages()
	if len(pkgs) == 0 || pkgs[0] != "vpl-gpu-rt" {
		t.Fatalf("pkgs = %v", pkgs)
	}
	cmd := d.InstallCommand("vpl-gpu-rt", "libvpl")
	if cmd != "sudo pacman -S vpl-gpu-rt libvpl" {
		t.Fatalf("cmd = %q", cmd)
	}
}

func TestParseOSReleaseDebian(t *testing.T) {
	path := filepath.Join("testdata", "os-release", "debian")
	d := platform.ParseOSRelease(path)
	if d.Family != platform.DistroFamilyDebian {
		t.Fatalf("family = %q", d.Family)
	}
	pkgs := d.QSVRuntimePackages()
	if len(pkgs) < 1 || pkgs[0] != "libmfx-gen1.2" {
		t.Fatalf("pkgs = %v", pkgs)
	}
}

func TestParseOSReleaseFedora(t *testing.T) {
	path := filepath.Join("testdata", "os-release", "fedora")
	d := platform.ParseOSRelease(path)
	if d.Family != platform.DistroFamilyRPM {
		t.Fatalf("family = %q", d.Family)
	}
	if pkgs := d.QSVRuntimePackages(); len(pkgs) == 0 || pkgs[0] != "intel-vpl-gpu-rt" {
		t.Fatalf("pkgs = %v", pkgs)
	}
}

func TestParseOSReleaseUbuntuIDLike(t *testing.T) {
	path := filepath.Join("testdata", "os-release", "ubuntu")
	d := platform.ParseOSRelease(path)
	if d.Family != platform.DistroFamilyDebian {
		t.Fatalf("family = %q", d.Family)
	}
}

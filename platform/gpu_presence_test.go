package platform

import (
	"os"
	"testing"
)

func TestDetectGPUPresenceFromSysfs(t *testing.T) {
	deps := gpuPresenceDeps{
		Glob: func(pattern string) ([]string, error) {
			if pattern == "/sys/class/drm/card*/device/vendor" {
				return []string{
					"/sys/class/drm/card0/device/vendor",
					"/sys/class/drm/card1/device/vendor",
				}, nil
			}
			return nil, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case "/sys/class/drm/card0/device/vendor":
				return []byte("0x8086\n"), nil
			case "/sys/class/drm/card1/device/vendor":
				return []byte("0x10de\n"), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		Stat: func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
	}
	p := detectGPUPresence(deps)
	if !p.Intel || !p.NVIDIA || p.AMD {
		t.Fatalf("presence=%+v", p)
	}
}

func TestRunningInContainerDockerenv(t *testing.T) {
	deps := containerDeps{
		Stat: func(path string) (os.FileInfo, error) {
			if path == "/.dockerenv" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		},
		ReadFile: func(string) ([]byte, error) { return nil, os.ErrNotExist },
	}
	if !runningInContainer(deps) {
		t.Fatal("expected container")
	}
}

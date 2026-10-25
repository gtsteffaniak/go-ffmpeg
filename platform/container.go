package platform

import (
	"os"
	"strings"
)

type containerDeps struct {
	Stat     func(name string) (os.FileInfo, error)
	ReadFile func(name string) ([]byte, error)
}

func defaultContainerDeps() containerDeps {
	return containerDeps{Stat: os.Stat, ReadFile: os.ReadFile}
}

// RunningInContainer reports docker/container-style isolation (/.dockerenv or cgroup).
func RunningInContainer() bool {
	return runningInContainer(defaultContainerDeps())
}

func runningInContainer(deps containerDeps) bool {
	if _, err := deps.Stat("/.dockerenv"); err == nil {
		return true
	}
	data, err := deps.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "docker") ||
		strings.Contains(lower, "containerd") ||
		strings.Contains(lower, "kubepods") ||
		strings.Contains(lower, "/pod")
}

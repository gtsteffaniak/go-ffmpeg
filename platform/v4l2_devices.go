package platform

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// V4L2M2MDevice is a discovered Video4Linux memory-to-memory node.
type V4L2M2MDevice struct {
	Path string
	Name string // sysfs card name when known
}

// ListV4L2M2MDevices returns /dev/video* nodes that advertise M2M capture/output.
// Discovery uses sysfs names (bcm2835-codec, rpivid, hevc-dec) when present.
func ListV4L2M2MDevices() []V4L2M2MDevice {
	matches, err := filepath.Glob("/dev/video*")
	if err != nil {
		return nil
	}
	sort.Strings(matches)
	var out []V4L2M2MDevice
	for _, node := range matches {
		if _, err := os.Stat(node); err != nil {
			continue
		}
		name := v4l2SysfsName(node)
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "codec") || strings.Contains(lower, "hevc") ||
			strings.Contains(lower, "rpivid") || strings.Contains(lower, "bcm2835") {
			out = append(out, V4L2M2MDevice{Path: node, Name: name})
		}
	}
	return out
}

func v4l2SysfsName(devPath string) string {
	base := filepath.Base(devPath)
	for _, pattern := range []string{
		"/sys/class/video4linux/" + base + "/name",
		"/sys/class/video4linux/" + base + "/device/name",
	} {
		if data, err := os.ReadFile(pattern); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

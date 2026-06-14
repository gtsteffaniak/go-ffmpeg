package platform_test

import (
	"runtime"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestDetect(t *testing.T) {
	info := platform.Detect()
	if info.OS != runtime.GOOS {
		t.Fatalf("os = %q", info.OS)
	}
	if info.Arch != runtime.GOARCH {
		t.Fatalf("arch = %q", info.Arch)
	}
}

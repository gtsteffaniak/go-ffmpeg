package platform_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestFilterDisplayControllerLines(t *testing.T) {
	lines := []string{
		"00:00.0 Host bridge: Intel Corporation Device 7a04",
		"00:01.0 PCI bridge: Intel Corporation Raptor Lake PCI Express Root Port",
		"00:02.0 VGA compatible controller [0300]: Intel Corporation Raptor Lake-P [UHD Graphics]",
		"01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GA107 [GeForce RTX 3050]",
		"02:00.0 USB controller: Intel Corporation Device 7a60",
	}
	got := platform.FilterDisplayControllerLines(lines)
	if len(got) != 2 {
		t.Fatalf("got %d GPUs, want 2: %v", len(got), got)
	}
	for _, line := range got {
		if !platform.IsDisplayControllerLineForTest(line) {
			t.Fatalf("expected display line, got %q", line)
		}
	}
}

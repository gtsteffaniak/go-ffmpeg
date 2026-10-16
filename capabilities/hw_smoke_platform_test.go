package capabilities

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestPlatformSkipReasonAMFNonAMD(t *testing.T) {
	msg := platformSkipReason("amf", platform.Info{Intel: true, NVIDIA: true})
	if !strings.Contains(msg, "no AMD GPU detected") {
		t.Fatalf("msg = %q", msg)
	}
	if strings.Contains(msg, "Intel/NVIDIA") {
		t.Fatalf("misleading wording: %q", msg)
	}
}

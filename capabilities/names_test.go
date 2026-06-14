package capabilities_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestAccelLabel(t *testing.T) {
	if got := capabilities.AccelLabel(capabilities.AccelQSV); got != "Intel Quick Sync Video" {
		t.Fatalf("got %q", got)
	}
}

func TestEncoderLabel(t *testing.T) {
	if got := capabilities.EncoderLabel("h264_qsv"); got != "H.264 — Intel Quick Sync" {
		t.Fatalf("got %q", got)
	}
}

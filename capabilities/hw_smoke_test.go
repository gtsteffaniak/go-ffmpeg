package capabilities

import (
	"strings"
	"testing"
)

func TestSummarizeHWError(t *testing.T) {
	msg := summarizeHWError("Unknown encoder 'h264_amf'")
	if msg != "encoder not available in this FFmpeg build" {
		t.Fatalf("got %q", msg)
	}
	msg = summarizeHWError("Error creating a MFX session: -9")
	if msg != "Intel QSV session failed (MFX -9)" {
		t.Fatalf("got %q", msg)
	}
}

func TestEncoderLabelKnown(t *testing.T) {
	if EncoderLabel("h264_vaapi") != "H.264 — VAAPI" {
		t.Fatal("unexpected label")
	}
}

func TestSummarizeHWErrorKeepsFailingTail(t *testing.T) {
	out := "Input #0, lavfi, from 'testsrc=duration=0.1:size=320x240:rate=30':\n  Duration: N/A\n  Stream #0:0: Video: wrapped_avframe\n" +
		"[h264_nvenc @ 0x1] OpenEncodeSessionEx failed: unsupported device (2)\nConversion failed!\n"
	got := summarizeHWError(out)
	if !strings.Contains(got, "OpenEncodeSessionEx failed") || strings.Contains(got, "Input #0") {
		t.Fatalf("summary = %q, want the failing tail", got)
	}
}

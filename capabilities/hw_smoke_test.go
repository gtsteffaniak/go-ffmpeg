package capabilities

import "testing"

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

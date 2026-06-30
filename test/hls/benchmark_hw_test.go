package main

import (
	"strings"
	"testing"
)

func hwEncoderFromName(encoder string) bool {
	return strings.Contains(encoder, "_qsv") || strings.Contains(encoder, "_vaapi") ||
		strings.Contains(encoder, "_nvenc") || strings.Contains(encoder, "_amf") ||
		strings.Contains(encoder, "_videotoolbox")
}

func TestHWLikelyActiveVideoToolboxEncoder(t *testing.T) {
	hv := hwVerification{
		ExpectedAccel: "videotoolbox",
		Encoder:       "h264_videotoolbox",
		HWEncoder:     hwEncoderFromName("h264_videotoolbox"),
	}
	if !hwLikelyActive(hv, resourceStats{}, true) {
		t.Fatal("expected HW active for h264_videotoolbox without GPU samples")
	}
}

func TestHWLikelyActiveSoftwareEncoderNeedsGPU(t *testing.T) {
	hv := hwVerification{
		ExpectedAccel: "videotoolbox",
		Encoder:       "libx264",
		HWEncoder:     hwEncoderFromName("libx264"),
	}
	gpu := 4.0
	res := resourceStats{GPUPercentAvg: &gpu}
	withGPU := hv
	withGPU.GPUDetected = true
	if !hwLikelyActive(withGPU, res, true) {
		t.Fatal("expected HW active when GPU util confirms activity")
	}
	if hwLikelyActive(hv, resourceStats{}, true) {
		t.Fatal("expected HW inactive when encoder is software and GPU is low")
	}
}

package encode_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestNormalizeFrameQuality(t *testing.T) {
	if encode.NormalizeFrameQuality(0) != encode.DefaultFrameQuality {
		t.Fatalf("zero quality default")
	}
	if encode.NormalizeFrameQuality(150) != 100 {
		t.Fatal("clamp high")
	}
}

func TestMJPEGQualityQScale(t *testing.T) {
	if encode.MJPEGQualityQScale(100) != "2" {
		t.Fatalf("best quality qscale")
	}
	if encode.MJPEGQualityQScale(1) != "31" {
		t.Fatalf("worst quality qscale")
	}
	if encode.MJPEGQualityQScale(85) != "6" {
		t.Fatalf("default-ish quality")
	}
}

func TestQualityToQScaleUsesMJPEGScale(t *testing.T) {
	if encode.QualityToQScale(85) != encode.MJPEGQualityQScale(85) {
		t.Fatal("alias")
	}
}

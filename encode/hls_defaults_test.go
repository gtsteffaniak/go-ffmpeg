package encode

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestDefaultHLSVideoProfile(t *testing.T) {
	t.Parallel()
	p1080 := DefaultHLSVideoProfile(1080)
	if p1080.Quality != PresetMedium {
		t.Fatalf("default quality = %q, want medium", p1080.Quality)
	}
	if p1080.Bitrate.Target != "5000k" {
		t.Fatalf("1080p target = %q, want 5000k", p1080.Bitrate.Target)
	}
	p720 := DefaultHLSVideoProfile(720)
	if p720.Bitrate.Target != "3500k" {
		t.Fatalf("720p target = %q, want 3500k", p720.Bitrate.Target)
	}
}

func TestHLSDecodeProfileForOnDemand(t *testing.T) {
	t.Parallel()
	out := HLSDecodeProfileForOnDemand(probe.StreamInfo{VideoCodec: "h264"})
	if !out.ForceSoftware {
		t.Fatal("on-demand should force software decode for h264")
	}
}

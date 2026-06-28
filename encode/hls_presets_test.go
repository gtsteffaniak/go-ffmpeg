package encode

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestNormalizeHLSPreset(t *testing.T) {
	t.Parallel()
	if NormalizeHLSPreset("optimized") != HLSPresetLowLatency {
		t.Fatal("optimized should map to low-latency")
	}
	if NormalizeHLSPreset("datasaver") != HLSPresetConstrained {
		t.Fatal("datasaver should map to constrained")
	}
}

func TestHLSVideoProfilePresets(t *testing.T) {
	t.Parallel()
	info := probe.StreamInfo{HasVideo: true, Height: 1080, VideoBitrate: 8_000_000}
	q := HLSVideoProfile(info, HLSPresetQuality, 0)
	if q.Quality != PresetMedium {
		t.Fatalf("quality preset = %q", q.Quality)
	}
	o := HLSVideoProfile(info, HLSPresetLowLatency, 0)
	if o.Quality != PresetVeryfast {
		t.Fatalf("low-latency preset = %q", o.Quality)
	}
}

func TestHLSDecodeProfileForOnDemand(t *testing.T) {
	t.Parallel()
	out := HLSDecodeProfileForOnDemand(probe.StreamInfo{VideoCodec: "h264"})
	if !out.ForceSoftware {
		t.Fatal("on-demand should force software decode for h264")
	}
}

func TestHLSConstrainedBitrateConfig(t *testing.T) {
	t.Parallel()
	info1080 := probe.StreamInfo{HasVideo: true, Height: 1080, VideoBitrate: 8_000_000}
	p := HLSVideoProfile(info1080, HLSPresetConstrained, 0)
	if p.Bitrate.Target != "800k" {
		t.Fatalf("720p constrained target = %q, want 800k", p.Bitrate.Target)
	}
	info480 := probe.StreamInfo{HasVideo: true, Height: 480, VideoBitrate: 2_000_000}
	p480 := HLSVideoProfile(info480, HLSPresetConstrained, 480)
	if p480.Bitrate.Target != "550k" {
		t.Fatalf("480p constrained target = %q, want 550k", p480.Bitrate.Target)
	}
}

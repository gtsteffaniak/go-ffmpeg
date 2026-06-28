package ops

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestUseVideoCopy(t *testing.T) {
	t.Parallel()
	eac3 := probe.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3", Height: 1080}
	pipeline := HLSPipelineOptions{Preset: encode.HLSPresetQuality}
	if !UseVideoCopy(eac3, pipeline) {
		t.Fatal("quality preset should use video copy for h264+eac3")
	}
	pipeline.Preset = encode.HLSPresetLowLatency
	if UseVideoCopy(eac3, pipeline) {
		t.Fatal("low-latency should not use video copy")
	}
}

func TestCanFMP4StreamCopy(t *testing.T) {
	t.Parallel()
	if !CanFMP4StreamCopy(probe.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "aac"}) {
		t.Fatal("h264+aac should remux")
	}
	if CanFMP4StreamCopy(probe.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3"}) {
		t.Fatal("h264+eac3 should not remux")
	}
}

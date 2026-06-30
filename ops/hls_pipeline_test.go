package ops

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestUseVideoCopy(t *testing.T) {
	t.Parallel()
	eac3 := probe.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3", Height: 1080}
	pipeline := HLSPipelineOptions{}
	if !UseVideoCopy(eac3, pipeline) {
		t.Fatal("default pipeline should use video copy for h264+eac3")
	}
	pipeline.ForceVideoTranscode = true
	if UseVideoCopy(eac3, pipeline) {
		t.Fatal("forced transcode should not use video copy")
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

func TestNeedsFullVideoTranscode(t *testing.T) {
	t.Parallel()
	h264 := probe.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "aac", Height: 1080}
	if NeedsFullVideoTranscode(h264, HLSPipelineOptions{MaxHeight: 1080}) {
		t.Fatal("h264 at max height should not require transcode")
	}
	if !NeedsFullVideoTranscode(h264, HLSPipelineOptions{ForceVideoTranscode: true}) {
		t.Fatal("forced transcode should require full encode")
	}
	hevc := probe.StreamInfo{HasVideo: true, VideoCodec: "hevc", Height: 1080}
	if !NeedsFullVideoTranscode(hevc, HLSPipelineOptions{}) {
		t.Fatal("hevc should require transcode")
	}
}

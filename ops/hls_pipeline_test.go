package ops

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestUseVideoCopy(t *testing.T) {
	t.Parallel()
	eac3MKV := probe.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3", FormatName: "matroska,webm", Height: 1080}
	if UseVideoCopy(eac3MKV, HLSPipelineOptions{}) {
		t.Fatal("mkv h264+eac3 must transcode video for MSE-safe fMP4")
	}
	eac3MP4 := probe.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3", FormatName: "mov,mp4,m4a,3gp,3g2", Height: 1080}
	if !UseVideoCopy(eac3MP4, HLSPipelineOptions{}) {
		t.Fatal("mp4 h264+eac3 should use video copy when audio transcodes")
	}
	pipeline := HLSPipelineOptions{ForceVideoTranscode: true}
	if UseVideoCopy(eac3MP4, pipeline) {
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

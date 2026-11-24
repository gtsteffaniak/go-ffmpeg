package ops

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestMotionNeedsMediaDuration(t *testing.T) {
	file := MotionDetectOptions{Input: InputSource{URL: "/v.mp4", StreamType: probe.StreamFile}}
	if !MotionNeedsMediaDuration(file) {
		t.Fatal("file inputs should probe duration when unbounded")
	}
	rtspBounded := MotionDetectOptions{
		Input:  InputSource{URL: "rtsp://x", StreamType: probe.StreamRTSP},
		EndSec: 10,
	}
	if MotionNeedsMediaDuration(rtspBounded) {
		t.Fatal("bounded live analysis must not require media duration")
	}
	rtspLive := MotionDetectOptions{Input: InputSource{URL: "rtsp://x", StreamType: probe.StreamRTSP}}
	if MotionNeedsMediaDuration(rtspLive) {
		t.Fatal("unbounded live inputs must not require media duration")
	}
}

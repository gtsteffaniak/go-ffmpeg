package probe

import "testing"

func TestApplyProbeStreamsUsesFirstMappedStreams(t *testing.T) {
	t.Parallel()

	info := StreamInfo{}
	applyProbeStreams(&info, []probeStreamEntry{
		{CodecType: "video", CodecName: "h264", Width: 1920, Height: 1080, BitRate: "5000000"},
		{CodecType: "audio", CodecName: "eac3"},
		{CodecType: "audio", CodecName: "aac"},
		{CodecType: "video", CodecName: "mpeg2video", Width: 720, Height: 480},
	})

	if !info.HasVideo || info.VideoCodec != "h264" {
		t.Fatalf("video = %+v, want h264", info)
	}
	if info.Width != 1920 || info.Height != 1080 {
		t.Fatalf("dimensions = %dx%d, want 1920x1080", info.Width, info.Height)
	}
	if info.VideoBitrate != 5000000 {
		t.Fatalf("video bitrate = %d, want 5000000", info.VideoBitrate)
	}
	if !info.HasAudio || info.AudioCodec != "eac3" {
		t.Fatalf("audio codec = %q, want eac3", info.AudioCodec)
	}
}

package capabilities

// encoderReportGroup defines one codec section in the capability report.
type encoderReportGroup struct {
	Title    string
	Codec    VideoCodec // empty for misc/audio group
	Encoders []string
}

var encoderReportGroups = []encoderReportGroup{
	{
		Title:    "H.264 / AVC",
		Codec:    CodecH264,
		Encoders: []string{"libx264", "h264_videotoolbox", "h264_nvenc", "h264_amf", "h264_qsv", "h264_vaapi"},
	},
	{
		Title:    "H.265 / HEVC",
		Codec:    CodecHEVC,
		Encoders: []string{"libx265", "libvvenc", "hevc_videotoolbox", "hevc_nvenc", "hevc_qsv", "hevc_vaapi"},
	},
	{
		Title:    "AV1",
		Codec:    CodecAV1,
		Encoders: []string{"libsvtav1", "librav1e", "libaom-av1", "av1_nvenc", "av1_amf", "av1_qsv", "av1_vaapi"},
	},
	{
		Title:    "VP9",
		Codec:    CodecVP9,
		Encoders: []string{"libvpx-vp9", "vp9_nvenc", "vp9_amf", "vp9_qsv", "vp9_vaapi"},
	},
	{
		Title: "Other encoders",
		Encoders: []string{
			"mjpeg", "aac", "libmp3lame",
		},
	},
}

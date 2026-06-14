package ops

import "github.com/gtsteffaniak/go-ffmpeg/capabilities"

func init() {
	Register(probeStreamOp{})
	Register(getMediaDurationOp{})
	Register(getImageDimensionsOp{})
	Register(screenshotOp{})
	Register(videoPreviewOp{})
	Register(transcodeOp{})
	Register(segmentRecordOp{})
	Register(fmp4StreamCopyOp{})
	Register(timelapseCompileOp{})
	Register(extractSubtitleOp{})
	Register(convertHEICOp{})
	Register(detectSubtitlesOp{})
}

type probeStreamOp struct{}

func (probeStreamOp) Name() string { return "ProbeStream" }
func (probeStreamOp) Requirements() RequirementSet {
	return RequirementSet{Protocols: []string{"file", "tcp"}}
}

type getMediaDurationOp struct{}

func (getMediaDurationOp) Name() string { return "GetMediaDuration" }
func (getMediaDurationOp) Requirements() RequirementSet {
	return RequirementSet{Protocols: []string{"file"}}
}

type getImageDimensionsOp struct{}

func (getImageDimensionsOp) Name() string { return "GetImageDimensions" }
func (getImageDimensionsOp) Requirements() RequirementSet {
	return RequirementSet{Protocols: []string{"file"}}
}

type screenshotOp struct{}

func (screenshotOp) Name() string { return "Screenshot" }
func (screenshotOp) Requirements() RequirementSet {
	return RequirementSet{Protocols: []string{"file", "tcp"}}
}

type videoPreviewOp struct{}

func (videoPreviewOp) Name() string { return "VideoPreview" }
func (videoPreviewOp) Requirements() RequirementSet {
	return RequirementSet{
		Encoders:  []string{"mjpeg"},
		Filters:   []string{"scale"},
		Protocols: []string{"file"},
	}
}

type transcodeOp struct{}

func (transcodeOp) Name() string { return "Transcode" }
func (transcodeOp) Requirements() RequirementSet {
	return RequirementSet{NeedsEncode: true, MinProfile: capabilities.BuildFull}
}

type segmentRecordOp struct{}

func (segmentRecordOp) Name() string { return "SegmentRecord" }
func (segmentRecordOp) Requirements() RequirementSet {
	return RequirementSet{
		NeedsEncode: true,
		MinProfile:  capabilities.BuildFull,
		Filters:     []string{"segment"},
	}
}

type fmp4StreamCopyOp struct{}

func (fmp4StreamCopyOp) Name() string { return "FMP4StreamCopy" }
func (fmp4StreamCopyOp) Requirements() RequirementSet {
	return RequirementSet{Protocols: []string{"tcp"}}
}

type timelapseCompileOp struct{}

func (timelapseCompileOp) Name() string { return "TimelapseCompile" }
func (timelapseCompileOp) Requirements() RequirementSet {
	return RequirementSet{
		NeedsEncode: true,
		MinProfile:  capabilities.BuildFull,
		Filters:     []string{"scale", "concat", "fps"},
	}
}

type extractSubtitleOp struct{}

func (extractSubtitleOp) Name() string { return "ExtractSubtitle" }
func (extractSubtitleOp) Requirements() RequirementSet {
	return RequirementSet{Protocols: []string{"file"}}
}

type convertHEICOp struct{}

func (convertHEICOp) Name() string { return "ConvertHEIC" }
func (convertHEICOp) Requirements() RequirementSet {
	return RequirementSet{
		Filters: []string{"scale", "tile", "transpose"},
	}
}

type detectSubtitlesOp struct{}

func (detectSubtitlesOp) Name() string { return "DetectSubtitles" }
func (detectSubtitlesOp) Requirements() RequirementSet {
	return RequirementSet{Protocols: []string{"file"}}
}

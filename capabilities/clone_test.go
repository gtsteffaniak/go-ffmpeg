package capabilities_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestCapabilitiesCloneIsIndependent(t *testing.T) {
	t.Parallel()
	orig := capabilities.NewCapabilities()
	orig.EnabledOps = []string{"ProbeStream"}
	orig.DisabledOps = map[string][]string{"Transcode": {"missing encoder"}}
	orig.CodecMatrix = map[capabilities.VideoCodec]capabilities.CodecSupport{
		capabilities.CodecH264: {
			Software: []string{"libx264"},
			Hardware: map[capabilities.AccelType]string{capabilities.AccelNVENC: "h264_nvenc"},
		},
	}

	copy := orig.Clone()
	copy.EnabledOps[0] = "mutated"
	copy.DisabledOps["Transcode"][0] = "mutated"
	copy.CodecMatrix[capabilities.CodecH264].Software[0] = "mutated"

	if orig.EnabledOps[0] != "ProbeStream" {
		t.Fatalf("orig EnabledOps mutated: %v", orig.EnabledOps)
	}
	if orig.DisabledOps["Transcode"][0] != "missing encoder" {
		t.Fatalf("orig DisabledOps mutated: %v", orig.DisabledOps)
	}
	if orig.CodecMatrix[capabilities.CodecH264].Software[0] != "libx264" {
		t.Fatalf("orig CodecMatrix mutated: %v", orig.CodecMatrix[capabilities.CodecH264].Software)
	}
}

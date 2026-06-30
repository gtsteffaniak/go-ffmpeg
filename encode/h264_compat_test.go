package encode_test

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestH264LevelForMaxHeight(t *testing.T) {
	t.Parallel()
	tests := []struct {
		height int
		want   string
	}{
		{480, "3.0"},
		{720, "3.1"},
		{1080, "4.0"},
		{1440, "4.1"},
		{0, "4.0"},
	}
	for _, tc := range tests {
		if got := encode.H264LevelForMaxHeight(tc.height); got != tc.want {
			t.Fatalf("height %d = %q, want %q", tc.height, got, tc.want)
		}
	}
}

func TestAppendH264FMP4CompatArgsVideoToolbox(t *testing.T) {
	t.Parallel()
	args := encode.AppendH264FMP4CompatArgs(nil, capabilities.AccelVideoToolbox, 1080)
	joined := strings.Join(args, " ")
	for _, want := range []string{"-profile:v", "baseline", "-level", "4.0", "-tag:v", "avc1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in compat args: %v", want, args)
		}
	}
}

func TestAppendH264FMP4CompatArgsVAAPI(t *testing.T) {
	t.Parallel()
	args := encode.AppendH264FMP4CompatArgs(nil, capabilities.AccelVAAPI, 1080)
	if len(args) != 2 || args[0] != "-tag:v" || args[1] != "avc1" {
		t.Fatalf("vaapi compat args = %v", args)
	}
}

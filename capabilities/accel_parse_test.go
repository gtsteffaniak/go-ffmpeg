package capabilities_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestParseAccelSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw      string
		want     capabilities.AccelType
		autoMode bool
	}{
		{"", "", true},
		{"auto", "", true},
		{"none", capabilities.AccelNone, false},
		{"software", capabilities.AccelNone, false},
		{"false", capabilities.AccelNone, false},
		{"qsv", capabilities.AccelQSV, false},
		{"NVENC", capabilities.AccelNVENC, false},
		{"vaapi", capabilities.AccelVAAPI, false},
		{"unknown-backend", "", true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			got, auto := capabilities.ParseAccelSelection(tc.raw)
			if got != tc.want || auto != tc.autoMode {
				t.Fatalf("ParseAccelSelection(%q) = (%q, %v), want (%q, %v)", tc.raw, got, auto, tc.want, tc.autoMode)
			}
		})
	}
}

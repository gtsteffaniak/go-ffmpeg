package capabilities

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestParseHWAccelKey(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		hwAccel string
		swCodec string
		ok      bool
	}{
		{"videotoolbox h264", "hwaccel:videotoolbox:h264", "videotoolbox", "h264", true},
		{"vaapi hevc", "hwaccel:vaapi:hevc", "vaapi", "hevc", true},
		{"not hwaccel", "h264", "", "", false},
		{"malformed", "hwaccel:videotoolbox", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hw, sw, ok := ParseHWAccelKey(tc.in)
			if ok != tc.ok || hw != tc.hwAccel || sw != tc.swCodec {
				t.Fatalf("ParseHWAccelKey(%q) = (%q, %q, %v), want (%q, %q, %v)", tc.in, hw, sw, ok, tc.hwAccel, tc.swCodec, tc.ok)
			}
		})
	}
}

func TestHWDecodeSmokeArgsVideoToolbox(t *testing.T) {
	cases := []struct {
		name     string
		decoder  string
		bitstream string
		swCodec  string
	}{
		{"h264", "hwaccel:videotoolbox:h264", "/tmp/test.h264", "h264"},
		{"vp9", "hwaccel:videotoolbox:vp9", "/tmp/test.ivf", "vp9"},
		{"av1", "hwaccel:videotoolbox:av1", "/tmp/test.mp4", "av1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := hwDecodeSmokeArgs(tc.decoder, tc.bitstream, "videotoolbox", tc.swCodec, platform.Info{})
			if args == nil {
				t.Fatal("expected videotoolbox decode smoke args")
			}
			joined := strings.Join(args, " ")
			for _, want := range []string{"-hwaccel", "videotoolbox", "-c:v", tc.swCodec, "-i", tc.bitstream} {
				if !strings.Contains(joined, want) {
					t.Fatalf("args %v missing %q", args, want)
				}
			}
		})
	}
}

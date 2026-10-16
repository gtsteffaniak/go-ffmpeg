package ops

import "testing"

func TestFMP4CopyVideoTag(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"hevc":  "hvc1",
		"H265":  "hvc1",
		"hev1":  "hvc1",
		"h264":  "",
		"av1":   "",
		" vp9 ": "",
	}
	for codec, want := range cases {
		if got := fmp4CopyVideoTag(codec); got != want {
			t.Fatalf("codec %q: tag = %q, want %q", codec, got, want)
		}
	}
}

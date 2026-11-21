package ops

import "testing"

func TestBitDepthFromPixFmt(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"yuv420p": 8, "nv12": 8, "yuv420p10le": 10, "p010le": 10, "yuv420p12le": 12, "": 0, "weird": 0,
	}
	for pf, want := range cases {
		if got := bitDepthFromPixFmt(pf); got != want {
			t.Errorf("bitDepthFromPixFmt(%q) = %d, want %d", pf, got, want)
		}
	}
}

func TestValidateConcatOptions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		opts    ConcatMediaOptions
		wantErr bool
	}{
		{"neither scale value is fine", ConcatMediaOptions{}, false},
		{"both scale values are fine", ConcatMediaOptions{Width: 640, Height: 360}, false},
		{"width alone is rejected", ConcatMediaOptions{Width: 640}, true},
		{"height alone is rejected", ConcatMediaOptions{Height: 360}, true},
		{"negative width is rejected", ConcatMediaOptions{Width: -1, Height: 360}, true},
		{"negative fps is rejected", ConcatMediaOptions{FPS: -1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateConcatOptions(tc.opts); (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %t", err, tc.wantErr)
			}
		})
	}
}

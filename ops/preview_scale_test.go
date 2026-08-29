package ops

import "testing"

func TestPreviewScaleFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		width  int
		height int
		mode   ScaleMode
		want   string
	}{
		{
			name:   "fit decrease",
			width:  640,
			height: 640,
			mode:   ScaleFit,
			want:   "scale=640:640:force_original_aspect_ratio=decrease",
		},
		{
			name:   "fill increase and crop",
			width:  256,
			height: 256,
			mode:   ScaleFill,
			want:   "scale=256:256:force_original_aspect_ratio=increase,crop=256:256",
		},
		{
			name:   "zero mode defaults to fit",
			width:  1024,
			height: 1024,
			want:   "scale=1024:1024:force_original_aspect_ratio=decrease",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := PreviewScaleFilter(tt.width, tt.height, tt.mode); got != tt.want {
				t.Fatalf("PreviewScaleFilter() = %q, want %q", got, tt.want)
			}
		})
	}
}

package ops

import "testing"

func TestResolvePreviewSeekSec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dur  float64
		opts PreviewOptions
		want float64
	}{
		{
			name: "explicit seek sec",
			dur:  100,
			opts: PreviewOptions{SeekSec: 25},
			want: 25,
		},
		{
			name: "seek sec clamped to duration",
			dur:  10,
			opts: PreviewOptions{SeekSec: 25},
			want: 10,
		},
		{
			name: "default ten percent",
			dur:  200,
			opts: PreviewOptions{},
			want: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ResolvePreviewSeekSec(tt.dur, tt.opts); got != tt.want {
				t.Fatalf("ResolvePreviewSeekSec() = %v, want %v", got, tt.want)
			}
		})
	}
}

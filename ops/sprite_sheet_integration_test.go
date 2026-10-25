//go:build integration

package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

func TestSpriteSheetIntegration(t *testing.T) {
	input := filepath.Join("..", "test", "data", "Big_Buck_Bunny_1080_10s_2MB.mp4")
	if _, err := os.Stat(input); err != nil {
		t.Skip(input)
	}
	ffmpegPath, ffprobePath := integrationFFmpegPaths(t)
	runner := &ffexec.Runner{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{SkipHWTests: true})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "sprite.jpg")
	res, err := SpriteSheet(ctx, runner, caps, SpriteSheetOptions{
		Input:       InputSource{URL: input, StreamType: probe.StreamFile},
		OutputPath:  out,
		Columns:     5,
		Rows:        2,
		IntervalSec: 2,
		TileWidth:   160,
		TileHeight:  90,
		DurationSec: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(out)
	if err != nil || st.Size() == 0 {
		t.Fatal("empty sprite")
	}
	if res.Columns != 5 || res.Rows != 2 {
		t.Fatalf("layout = %dx%d", res.Columns, res.Rows)
	}
}

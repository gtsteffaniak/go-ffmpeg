// Sprite sheet builds a tiled JPEG thumbnail strip and WebVTT cues.
//
// Usage:
//
//	GOFFMPEG_SAMPLE_MP4=video.mp4 go run ./examples/sprite-sheet
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func main() {
	input := flag.String("input", envOr("GOFFMPEG_SAMPLE_MP4", "video.mp4"), "input video")
	out := flag.String("output", "sprite.jpg", "sprite JPEG path")
	cols := flag.Int("columns", 10, "tile columns")
	rows := flag.Int("rows", 10, "tile rows")
	interval := flag.Float64("interval", 1, "seconds between tiles")
	flag.Parse()

	ctx := context.Background()
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{
		FFmpegPath:  os.Getenv("GOFFMPEG_FFMPEG_PATH"),
		FFprobePath: os.Getenv("GOFFMPEG_FFPROBE_PATH"),
		SkipHWTests: os.Getenv("GOFFMPEG_SKIP_HW") == "1",
	})
	if err != nil {
		log.Fatal(err)
	}

	caps := svc.SpriteSheetCapabilities()
	log.Printf("sprite hardware decode available: %v (preferred %s)", caps.HardwareDecodeAvailable, caps.PreferredDecode.Label)

	vtt := filepath.Join(filepath.Dir(*out), "thumbs.vtt")
	res, err := svc.SpriteSheet(ctx, ffmpeg.SpriteSheetOptions{
		Input:       ffmpeg.InputSource{URL: *input, StreamType: ffmpeg.StreamFile},
		OutputPath:  *out,
		WebVTTPath:  vtt,
		Columns:     *cols,
		Rows:        *rows,
		IntervalSec: *interval,
		TileWidth:   160,
		TileHeight:  90,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%dx%d tiles, interval %.2fs)", res.OutputPath, res.Columns, res.Rows, res.IntervalSec)
	if res.WebVTTPath != "" {
		log.Printf("wrote %s", res.WebVTTPath)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

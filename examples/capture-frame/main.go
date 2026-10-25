// Capture-frame extracts a single JPEG still using the unified capture API.
//
// Usage:
//
//	GOFFMPEG_SAMPLE_MP4=video.mp4 go run ./examples/capture-frame
//	GOFFMPEG_SAMPLE_MP4=video.mp4 CAPTURE_OUT=frame.jpg go run ./examples/capture-frame -at-percent 10
package main

import (
	"context"
	"flag"
	"log"
	"os"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func main() {
	input := flag.String("input", envOr("GOFFMPEG_SAMPLE_MP4", "video.mp4"), "input video path")
	output := flag.String("output", envOr("CAPTURE_OUT", "frame.jpg"), "output image path")
	quality := flag.Int("quality", 85, "JPEG quality 1-100")
	atSec := flag.Float64("at-sec", 0, "seek to timestamp in seconds (0 = first frame)")
	atPercent := flag.Float64("at-percent", 0, "seek to percent of duration (requires ffprobe unless -duration set)")
	duration := flag.Float64("duration", 0, "known media duration in seconds (skips ffprobe for percent seeks)")
	exact := flag.Bool("exact", false, "frame-accurate seek (slower)")
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

	var at ffmpeg.FramePosition
	switch {
	case *atPercent > 0:
		at = ffmpeg.AtPercent(*atPercent)
	case *atSec > 0:
		at = ffmpeg.AtSec(*atSec)
	}

	opts := ffmpeg.FrameOptions{
		Input:       ffmpeg.InputSource{URL: *input, StreamType: ffmpeg.StreamFile},
		At:          at,
		DurationSec: *duration,
		Quality:     *quality,
	}
	if *exact {
		opts.Accuracy = ffmpeg.CaptureExact
	}

	if err := svc.CaptureFrameFile(ctx, *output, opts); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s", *output)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

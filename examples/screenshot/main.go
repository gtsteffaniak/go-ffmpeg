// Screenshot extracts a single JPEG frame from a video file.
//
// Usage:
//
//	GOFFMPEG_SAMPLE_MP4=video.mp4 go run ./examples/screenshot
//	GOFFMPEG_SAMPLE_MP4=video.mp4 SCREENSHOT_OUT=frame.jpg go run ./examples/screenshot
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
	output := flag.String("output", envOr("SCREENSHOT_OUT", "frame.jpg"), "output JPEG path")
	quality := flag.Int("quality", 85, "JPEG quality 1-100")
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

	if err := svc.Screenshot(ctx, ffmpeg.ScreenshotOptions{
		Input:      ffmpeg.InputSource{URL: *input, StreamType: ffmpeg.StreamFile},
		OutputPath: *output,
		Quality:    *quality,
	}); err != nil {
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

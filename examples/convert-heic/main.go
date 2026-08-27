// Convert-heic decodes a HEIC/HEIF image to JPEG.
//
// Usage:
//
//	INPUT=photo.heic OUTPUT=photo.jpg go run ./examples/convert-heic
package main

import (
	"context"
	"flag"
	"log"
	"os"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func main() {
	input := flag.String("input", envOr("INPUT", "photo.heic"), "input HEIC path")
	output := flag.String("output", envOr("OUTPUT", "photo.jpg"), "output JPEG path")
	quality := flag.Int("quality", 90, "JPEG quality 1-100")
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

	if err := svc.ConvertHEIC(ctx, ffmpeg.ConvertHEICOptions{
		InputPath:  *input,
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

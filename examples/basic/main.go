package main

import (
	"context"
	"fmt"
	"log"
	"os"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func main() {
	ctx := context.Background()
	skipHW := os.Getenv("GOFFMPEG_SKIP_HW") == "1"
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{
		FFmpegPath:  os.Getenv("GOFFMPEG_FFMPEG_PATH"),
		SkipHWTests: skipHW,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(svc.Capabilities().ReportString())
	fmt.Println("Enabled operations:", svc.SupportedOps())

	if path := os.Getenv("GOFFMPEG_SAMPLE_MP4"); path != "" {
		dur, err := svc.GetMediaDuration(ctx, path)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Duration of %s: %.2fs\n", path, dur)
	}
}

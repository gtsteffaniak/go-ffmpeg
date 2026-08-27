// Transcode remuxes or re-encodes a file to MP4 with a VideoProfile.
//
// Usage:
//
//	INPUT=input.mkv OUTPUT=output.mp4 go run ./examples/transcode
package main

import (
	"context"
	"flag"
	"log"
	"os"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func main() {
	input := flag.String("input", envOr("INPUT", "input.mkv"), "input media path")
	output := flag.String("output", envOr("OUTPUT", "output.mp4"), "output MP4 path")
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

	if err := svc.Transcode(ctx, ffmpeg.TranscodeOptions{
		Input:      ffmpeg.InputSource{URL: *input, StreamType: ffmpeg.StreamFile},
		OutputPath: *output,
		Profile: ffmpeg.VideoProfile{
			Codec: ffmpeg.CodecH264,
		},
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

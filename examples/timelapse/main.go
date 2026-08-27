// Timelapse compiles a concat demuxer list of still images into an MP4.
//
// Create a concat list (one file per line: file 'frame001.jpg'):
//
//	printf "file 'frame001.jpg'\nfile 'frame002.jpg'\n" > frames.txt
//
// Usage:
//
//	TIMELAPSE_LIST=frames.txt OUTPUT=timelapse.mp4 go run ./examples/timelapse
package main

import (
	"context"
	"flag"
	"log"
	"os"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func main() {
	list := flag.String("list", envOr("TIMELAPSE_LIST", "frames.txt"), "ffmpeg concat demuxer list file")
	output := flag.String("output", envOr("OUTPUT", "timelapse.mp4"), "output MP4 path")
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

	if err := svc.TimelapseCompile(ctx, ffmpeg.TimelapseCompileOptions{
		ConcatListPath: *list,
		OutputPath:     *output,
		Profile:        ffmpeg.VideoProfile{Codec: ffmpeg.CodecH264},
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

// HLS writes one on-demand fMP4 segment (init + media) for browser MSE playback.
//
// Usage:
//
//	GOFFMPEG_SAMPLE_MP4=video.mp4 HLS_OUT=seg0.m4s go run ./examples/hls
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
	output := flag.String("output", envOr("HLS_OUT", "segment0.m4s"), "output fMP4 fragment path")
	start := flag.Float64("start", 0, "seek position in seconds")
	duration := flag.Float64("duration", 4, "segment duration in seconds")
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

	init, media, err := svc.HLSInitAndSegment(ctx, ffmpeg.HLSSegmentOptions{
		Input:            ffmpeg.InputSource{URL: *input, StreamType: ffmpeg.StreamFile},
		StartSec:         *start,
		MediaTimelineSec: *start,
		DurationSec:      *duration,
		Profile:          ffmpeg.VideoProfile{Codec: ffmpeg.CodecH264},
	})
	if err != nil {
		log.Fatal(err)
	}

	data := append(append([]byte(nil), init...), media...)
	if err := os.WriteFile(*output, data, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%d bytes init + %d bytes media)", *output, len(init), len(media))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package ffmpeg_test

import (
	"context"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func ExampleNew() {
	ctx := context.Background()
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{
		FFmpegPath: "/usr/local/bin",
	})
	if err != nil {
		panic(err)
	}
	_ = svc.SupportedOps()
}

func ExampleService_ProbeFile() {
	ctx := context.Background()
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{})
	if err != nil {
		panic(err)
	}
	info, err := svc.ProbeFile(ctx, "video.mp4")
	if err != nil {
		panic(err)
	}
	_ = info.VideoCodec
}

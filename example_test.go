package ffmpeg_test

import (
	"context"
	"io"
	"os"

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

func ExampleService_Screenshot() {
	ctx := context.Background()
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{SkipHWTests: true})
	if err != nil {
		panic(err)
	}
	err = svc.Screenshot(ctx, ffmpeg.ScreenshotOptions{
		Input:      ffmpeg.InputSource{URL: "video.mp4", StreamType: ffmpeg.StreamFile},
		OutputPath: "frame.jpg",
		Quality:    85,
	})
	if err != nil {
		panic(err)
	}
}

func ExampleService_Transcode() {
	ctx := context.Background()
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{SkipHWTests: true})
	if err != nil {
		panic(err)
	}
	err = svc.Transcode(ctx, ffmpeg.TranscodeOptions{
		Input:      ffmpeg.InputSource{URL: "input.mkv", StreamType: ffmpeg.StreamFile},
		OutputPath: "output.mp4",
		Profile: ffmpeg.VideoProfile{
			Codec: ffmpeg.CodecH264,
		},
	})
	if err != nil {
		panic(err)
	}
}

func ExampleService_HLSSegmentMedia() {
	ctx := context.Background()
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{SkipHWTests: true})
	if err != nil {
		panic(err)
	}
	err = svc.HLSSegmentMedia(ctx, io.Discard, ffmpeg.HLSSegmentOptions{
		Input:            ffmpeg.InputSource{URL: "video.mp4", StreamType: ffmpeg.StreamFile},
		StartSec:         4,
		MediaTimelineSec: 4,
		DurationSec:      4,
		Profile:          ffmpeg.VideoProfile{Codec: ffmpeg.CodecH264},
	})
	if err != nil {
		panic(err)
	}
}

func ExampleService_TimelapseCompile() {
	ctx := context.Background()
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{SkipHWTests: true})
	if err != nil {
		panic(err)
	}
	listPath := os.Getenv("TIMELAPSE_LIST")
	if listPath == "" {
		listPath = "frames.txt"
	}
	err = svc.TimelapseCompile(ctx, ffmpeg.TimelapseCompileOptions{
		ConcatListPath: listPath,
		OutputPath:     "timelapse.mp4",
		Profile:        ffmpeg.VideoProfile{Codec: ffmpeg.CodecH264},
	})
	if err != nil {
		panic(err)
	}
}

func ExampleService_ConvertHEIC() {
	ctx := context.Background()
	svc, err := ffmpeg.New(ctx, ffmpeg.Config{SkipHWTests: true})
	if err != nil {
		panic(err)
	}
	err = svc.ConvertHEIC(ctx, ffmpeg.ConvertHEICOptions{
		InputPath:  "photo.heic",
		OutputPath: "photo.jpg",
		Quality:    90,
	})
	if err != nil {
		panic(err)
	}
}

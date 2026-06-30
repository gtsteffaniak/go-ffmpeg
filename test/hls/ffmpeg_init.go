package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

var (
	ffmpegInitOnce sync.Once
	ffmpegInitErr  error
	ffmpegSvc      *goffmpeg.Service
	ffmpegDebug    bool
)

// initFFmpeg detects hardware once and reuses the service for all matrix/benchmark runs.
func initFFmpeg(ctx context.Context, debug bool) (*goffmpeg.Service, error) {
	ffmpegInitOnce.Do(func() {
		ffmpegDebug = debug
		ffmpegSvc, ffmpegInitErr = goffmpeg.New(ctx, goffmpeg.Config{
			FFmpegPath:    os.Getenv("GOFFMPEG_FFMPEG_PATH"),
			FFprobePath:   os.Getenv("GOFFMPEG_FFPROBE_PATH"),
			MaxConcurrent: 2,
			SkipHWTests:   os.Getenv("GOFFMPEG_SKIP_HW") == "1",
			VerboseFFmpeg: debug,
		})
	})
	if ffmpegInitErr != nil {
		return nil, ffmpegInitErr
	}
	if ffmpegSvc == nil {
		return nil, fmt.Errorf("ffmpeg service unavailable")
	}
	return ffmpegSvc, nil
}

func cachedCapabilities(svc *goffmpeg.Service) *capabilities.Capabilities {
	if svc == nil {
		return nil
	}
	return svc.Capabilities()
}

func encodeAccelVariants(caps *capabilities.Capabilities) []capabilities.AccelType {
	if caps == nil {
		return []capabilities.AccelType{capabilities.AccelNone}
	}
	out := []capabilities.AccelType{capabilities.AccelNone}
	hierarchy := capabilities.HierarchyForPlatform(caps.Platform)
	for _, accel := range hierarchy {
		if caps.CodecEncodeAvailable(capabilities.CodecH264, accel) {
			out = append(out, accel)
		}
	}
	return out
}

func onDemandDefaults() goffmpeg.OnDemandHLSDefaults {
	return goffmpeg.DefaultOnDemandHLSDefaults()
}

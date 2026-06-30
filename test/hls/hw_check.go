package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func runHWCheck(args []string) int {
	fs := flag.NewFlagSet("hw-check", flag.ExitOnError)
	file := fs.String("file", envOr("HLS_TEST_FILE", defaultSampleVideo()), "input file")
	accel := fs.String("accel", "qsv", "qsv, vaapi, or software")
	_ = fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	info, _ := svc.ProbeFile(ctx, *file)
	defaults := onDemandDefaults()
	params := goffmpeg.HLSSegmentParams{MaxHeight: 1080, GOP: defaults.DefaultGOP, Decode: encode.VideoDecodeProfile{Codec: encode.CodecH264}, Profile: encode.VideoProfile{Codec: encode.CodecH264, GOP: defaults.DefaultGOP}}
	switch *accel {
	case "software":
		params.Profile.ForceSoftware = true
	case "vaapi":
		params.Profile.Accel = capabilities.AccelVAAPI
		params.Decode.Accel = capabilities.AccelVAAPI
	case "qsv":
		params.Profile.Accel = capabilities.AccelQSV
		params.Decode.Accel = capabilities.AccelQSV
	}
	fmt.Println("plan:", svc.DescribeHLSSegmentPlan(params))
	starts, durs := goffmpeg.BuildHLSSegmentTimeline(info.Duration, nil, goffmpeg.DefaultHLSSegmentDurationSec)
	kf, _ := svc.ProbeVideoKeyframeTimes(ctx, *file)
	kfs := goffmpeg.SanitizeHLSKeyframes(kf, info.Duration)
	opts := goffmpeg.BuildHLSSegmentOptions(*file, 0, params, starts, durs, false, kfs, goffmpeg.DefaultHLSSegmentDurationSec)
	monitor := newResourceMonitor()
	monitor.Start()
	t0 := time.Now()
	_, _, err = svc.HLSInitAndSegment(ctx, opts)
	res := monitor.Stop()
	fmt.Printf("encode_ms=%d cpu_avg=%.0f%% gpu_monitor=%s", time.Since(t0).Milliseconds(), res.CPUPercentAvg, res.GPUMonitor)
	if res.GPUPercentAvg != nil {
		fmt.Printf(" gpu_avg=%.0f%%", *res.GPUPercentAvg)
	}
	fmt.Println()
	if err != nil {
		fmt.Println("FAIL", err)
		return 1
	}
	fmt.Println("PASS")
	return 0
}

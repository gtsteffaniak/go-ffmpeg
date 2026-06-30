package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

type hlsCheckReport struct {
	Pass     bool                `json:"pass"`
	File     string              `json:"file"`
	Mode     string              `json:"mode"`
	Segments []segmentReport     `json:"segments"`
	Issues   []mp4.TimelineIssue `json:"issues"`
}

type segmentReport struct {
	Index            int     `json:"index"`
	ExpectedStartSec float64 `json:"expectedStartSec"`
	MediaStartSec    float64 `json:"mediaStartSec"`
	ExpectedDurSec   float64 `json:"expectedDurSec"`
	ActualDurSec     float64 `json:"actualDurSec"`
	Bytes            int     `json:"bytes"`
}

func runHLSCheck(args []string) int {
	fs := flag.NewFlagSet("hls-check", flag.ExitOnError)
	file := fs.String("file", envOr("HLS_TEST_FILE", defaultSampleVideo()), "input media file")
	segments := fs.Int("segments", 5, "number of segments to encode and validate")
	mode := fs.String("mode", "remux", "encode mode: remux, copy, transcode")
	debug := fs.Bool("debug", false, "stream ffmpeg stderr to terminal")
	jsonOut := fs.String("json", "", "write JSON report to path")
	tolerance := fs.Float64("tolerance", mp4.DefaultHLSTimeToleranceSec, "timeline tolerance in seconds")
	_ = fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	svc, err := initFFmpeg(ctx, *debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg init: %v\n", err)
		return 1
	}

	report, err := checkHLS(ctx, svc, *file, *mode, *segments, *tolerance)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hls-check: %v\n", err)
		return 1
	}

	printHumanReport(report)
	if *jsonOut != "" {
		if err := writeJSON(*jsonOut, report); err != nil {
			fmt.Fprintf(os.Stderr, "write json: %v\n", err)
			return 1
		}
	}
	if !report.Pass {
		return 1
	}
	return 0
}

type encodedHLSSegment struct {
	Media  []byte
	Report segmentReport
}

type encodedHLSResult struct {
	Init     []byte
	Segments []encodedHLSSegment
	Report   *hlsCheckReport
}

func encodeHLS(ctx context.Context, svc *goffmpeg.Service, file, mode string, segCount int, tolerance float64) (*encodedHLSResult, error) {
	if _, err := os.Stat(file); err != nil {
		return nil, fmt.Errorf("file: %w", err)
	}

	info, err := svc.ProbeFile(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("probe: %w", err)
	}

	params, err := paramsForMode(ctx, svc, file, info, mode)
	if err != nil {
		return nil, err
	}

	keyframes, _ := svc.ProbeVideoKeyframeTimes(ctx, file)
	keyframeSeekTimes := goffmpeg.SanitizeHLSKeyframes(keyframes, info.Duration)
	starts, durations := goffmpeg.BuildHLSSegmentTimeline(info.Duration, keyframeSeekTimes, goffmpeg.DefaultHLSSegmentDurationSec)
	if segCount > len(starts) {
		segCount = len(starts)
	}
	if segCount < 1 {
		segCount = 1
	}

	report := &hlsCheckReport{
		Pass: true,
		File: file,
		Mode: mode,
	}

	result := &encodedHLSResult{Report: report}
	var mediaEnds []float64
	var prevSeg *segmentReport
	var trackTimescales map[uint32]uint32

	for i := 0; i < segCount; i++ {
		if i < len(starts) && starts[i] >= info.Duration-0.01 {
			break
		}
		mediaTimelineSec := 0.0
		if i > 0 && len(mediaEnds) >= i {
			mediaTimelineSec = mediaEnds[i-1]
		} else if i < len(starts) {
			mediaTimelineSec = starts[i]
		}

		opts := goffmpeg.BuildHLSSegmentOptions(file, i, params, starts, durations, false, keyframeSeekTimes, goffmpeg.DefaultHLSSegmentDurationSec)
		opts.MediaTimelineSec = mediaTimelineSec

		var media []byte
		if i == 0 {
			result.Init, media, err = svc.HLSInitAndSegment(ctx, opts)
			if len(result.Init) > 0 {
				trackTimescales = mp4.TrackTimescalesFromInit(result.Init)
			}
		} else {
			var buf bytes.Buffer
			err = svc.HLSSegmentMedia(ctx, &buf, opts)
			media = buf.Bytes()
		}
		if err != nil {
			return nil, fmt.Errorf("encode segment %d: %w", i, err)
		}

		expectedDur := goffmpeg.DefaultHLSSegmentDurationSec
		if i < len(durations) {
			expectedDur = durations[i]
		}

		startSec, _ := mp4.FragmentMediaStartSec(media)
		actualDur := mp4.FragmentDurationSecWithTimescales(media, trackTimescales)
		if i > 0 && len(report.Segments) > 0 {
			implied := startSec - report.Segments[len(report.Segments)-1].MediaStartSec
			if implied > actualDur {
				actualDur = implied
			}
		}
		if actualDur <= 0 {
			actualDur = expectedDur
		}
		end := startSec + actualDur
		if len(mediaEnds) <= i {
			mediaEnds = append(mediaEnds, end)
		} else {
			mediaEnds[i] = end
		}

		seg := segmentReport{
			Index:            i,
			ExpectedStartSec: mediaTimelineSec,
			MediaStartSec:    startSec,
			ExpectedDurSec:   expectedDur,
			ActualDurSec:     actualDur,
			Bytes:            len(media),
		}
		report.Segments = append(report.Segments, seg)
		result.Segments = append(result.Segments, encodedHLSSegment{Media: media, Report: seg})

		issues := mp4.ValidateSegmentTimeline(media, mp4.SegmentTimeline{
			Index:            i,
			ExpectedStartSec: mediaTimelineSec,
			ExpectedDurSec:   expectedDur,
			MediaStartSec:    startSec,
			ActualDurSec:     actualDur,
			Bytes:            len(media),
		}, tolerance)
		issues = filterKeyframeAlignedDurationIssues(issues, actualDur, expectedDur, mediaTimelineSec, keyframeSeekTimes, tolerance)
		if prevSeg != nil {
			prevTimeline := mp4.SegmentTimeline{
				Index:         prevSeg.Index,
				MediaStartSec: prevSeg.MediaStartSec,
				ActualDurSec:  prevSeg.ActualDurSec,
			}
			nextTimeline := mp4.SegmentTimeline{
				Index:         seg.Index,
				MediaStartSec: seg.MediaStartSec,
			}
			issues = append(issues, mp4.ValidateContinuity(prevTimeline, nextTimeline, tolerance)...)
		}
		if len(issues) > 0 {
			report.Pass = false
			report.Issues = append(report.Issues, issues...)
		}
		prevSeg = &report.Segments[len(report.Segments)-1]
	}

	return result, nil
}

func checkHLS(ctx context.Context, svc *goffmpeg.Service, file, mode string, segCount int, tolerance float64) (*hlsCheckReport, error) {
	result, err := encodeHLS(ctx, svc, file, mode, segCount, tolerance)
	if err != nil {
		return nil, err
	}
	return result.Report, nil
}

func paramsForMode(ctx context.Context, svc *goffmpeg.Service, path string, info goffmpeg.StreamInfo, mode string) (goffmpeg.HLSSegmentParams, error) {
	defaults := onDemandDefaults()
	switch mode {
	case "remux":
		return goffmpeg.HLSSegmentParams{Remux: true, GOP: defaults.DefaultGOP}, nil
	case "copy":
		return goffmpeg.HLSSegmentParams{VideoCopy: true, GOP: defaults.DefaultGOP}, nil
	case "transcode":
		gop := goffmpeg.HLSSegmentGOP(30, defaults)
		if fps, err := svc.ProbeVideoFPS(ctx, path); err == nil {
			gop = goffmpeg.HLSSegmentGOP(fps, defaults)
		}
		return goffmpeg.HLSSegmentParams{
			Remux:     false,
			VideoCopy: false,
			MaxHeight: 1080,
			GOP:       gop,
			Decode:    encode.OnDemandHLSDecodeProfile(encode.VideoDecodeProfile{Codec: videoCodecFromProbe(info.VideoCodec)}),
			Profile:   encode.VideoProfile{Codec: encode.CodecH264, Quality: encode.PresetVeryfast, GOP: gop},
		}, nil
	default:
		return goffmpeg.HLSSegmentParams{}, fmt.Errorf("unknown mode %q (use remux, copy, transcode)", mode)
	}
}

func printHumanReport(r *hlsCheckReport) {
	status := "PASS"
	if !r.Pass {
		status = "FAIL"
	}
	fmt.Printf("%s  file=%s mode=%s segments=%d issues=%d\n", status, r.File, r.Mode, len(r.Segments), len(r.Issues))
	for _, seg := range r.Segments {
		fmt.Printf("  seg %d: start=%.3f (exp %.3f) dur=%.3f (exp %.3f) bytes=%d\n",
			seg.Index, seg.MediaStartSec, seg.ExpectedStartSec, seg.ActualDurSec, seg.ExpectedDurSec, seg.Bytes)
	}
	for _, issue := range r.Issues {
		fmt.Printf("  [%s] %s\n", issue.Check, issue.Message)
	}
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

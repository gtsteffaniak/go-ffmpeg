package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/mp4"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

type continuousCheckReport struct {
	Pass           bool                `json:"pass"`
	File           string              `json:"file"`
	Mode           string              `json:"mode"`
	OutputDir      string              `json:"outputDir"`
	SegmentCount   int                 `json:"segmentCount"`
	PlaylistOK     bool                `json:"playlistOk"`
	HasEndList     bool                `json:"hasEndList"`
	SourceHasAudio bool                `json:"sourceHasAudio"`
	InitBytes      int                 `json:"initBytes"`
	TotalEncodeMs  int64               `json:"totalEncodeMs"`
	Segments       []segmentReport     `json:"segments"`
	Issues         []mp4.TimelineIssue `json:"issues"`
	AudioIssues    []string            `json:"audioIssues,omitempty"`
}

func runContinuousCheck(args []string) int {
	fs := flag.NewFlagSet("continuous-check", flag.ExitOnError)
	file := fs.String("file", envOr("HLS_TEST_FILE", defaultSampleVideo()), "input media file")
	mode := fs.String("mode", "remux", "encode mode: remux, copy, transcode")
	startIndex := fs.Int("start-index", 0, "HLS start_number / resume segment index")
	startSec := fs.Float64("start-sec", 0, "ffmpeg input seek seconds (0 = derive from segment grid)")
	debug := fs.Bool("debug", false, "stream ffmpeg stderr to terminal")
	jsonOut := fs.String("json", "", "write JSON report to path")
	tolerance := fs.Float64("tolerance", mp4.DefaultHLSTimeToleranceSec, "timeline tolerance in seconds")
	outDir := fs.String("out", "", "output directory (default: temp dir)")
	_ = fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	svc, err := initFFmpeg(ctx, *debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg init: %v\n", err)
		return 1
	}

	report, err := checkContinuousHLS(ctx, svc, *file, *mode, *outDir, *tolerance, *startIndex, *startSec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "continuous-check: %v\n", err)
		return 1
	}
	if *outDir == "" && report.OutputDir != "" {
		defer os.RemoveAll(report.OutputDir)
	}

	printContinuousReport(report)
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

func checkContinuousHLS(ctx context.Context, svc *goffmpeg.Service, file, mode, outDir string, tolerance float64, startIndex int, startSec float64) (*continuousCheckReport, error) {
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

	if outDir == "" {
		outDir, err = os.MkdirTemp("", "hls-continuous-check-*")
		if err != nil {
			return nil, err
		}
	} else if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	keyframes, _ := svc.ProbeVideoKeyframeTimes(ctx, file)
	keyframeSeekTimes := goffmpeg.SanitizeHLSKeyframes(keyframes, info.Duration)
	starts, durations := goffmpeg.BuildHLSSegmentTimeline(info.Duration, keyframeSeekTimes, goffmpeg.DefaultHLSSegmentDurationSec)
	opts := continuousOptions(file, params, outDir, starts, durations, mode == "transcode", startIndex, startSec)
	start := time.Now()
	job, err := svc.StartHLSContinuous(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("start continuous: %w", err)
	}
	if err := job.Wait(); err != nil {
		return nil, fmt.Errorf("continuous job: %w", err)
	}
	totalMs := time.Since(start).Milliseconds()

	if err := finalizeContinuousOutputDir(outDir); err != nil {
		return nil, fmt.Errorf("finalize playlist: %w", err)
	}

	playlistPath := filepath.Join(outDir, "ffmpeg.m3u8")
	playlist, err := parseFFmpegM3U8(playlistPath)
	if err != nil {
		return nil, fmt.Errorf("parse playlist: %w", err)
	}

	report := &continuousCheckReport{
		Pass:           true,
		File:           file,
		Mode:           mode,
		OutputDir:      outDir,
		HasEndList:     playlist.HasEndList,
		SourceHasAudio: info.HasAudio,
		TotalEncodeMs:  totalMs,
	}

	initPath := resolveContinuousInitPath(outDir, playlist.InitURI)
	initBytes, err := os.ReadFile(initPath)
	if err != nil {
		report.Pass = false
		report.Issues = append(report.Issues, mp4.TimelineIssue{Check: "init", Message: err.Error()})
	} else {
		report.InitBytes = len(initBytes)
		if audioIssues := validateInitTracks(initBytes, info.HasAudio); len(audioIssues) > 0 {
			report.Pass = false
			report.AudioIssues = audioIssues
		}
	}

	trackTimescales := mp4.TrackTimescalesFromInit(initBytes)
	var prevSeg *segmentReport
	playlistOK := true
	exactTranscodeTimeline := mode == "transcode"

	for i, plSeg := range playlist.Segments {
		segPath := resolveContinuousSegmentPath(outDir, plSeg.URI)
		media, err := os.ReadFile(segPath)
		if err != nil {
			report.Pass = false
			playlistOK = false
			report.Issues = append(report.Issues, mp4.TimelineIssue{
				Check:   "playlist_file",
				Message: fmt.Sprintf("segment %d missing on disk: %s", i, segPath),
			})
			continue
		}

		absIndex := opts.StartIndex + i
		expectedStart := 0.0
		expectedDur := goffmpeg.DefaultHLSSegmentDurationSec
		if absIndex >= 0 && absIndex < len(durations) {
			expectedDur = durations[absIndex]
		}
		if exactTranscodeTimeline {
			expectedStart = float64(absIndex) * goffmpeg.DefaultHLSSegmentDurationSec
			if plSeg.DurSec > 0 {
				expectedDur = plSeg.DurSec
			}
		} else {
			for j := 0; j < absIndex && j < len(durations); j++ {
				expectedStart += durations[j]
			}
		}

		startSec, _ := mp4.FragmentMediaStartSec(media)
		actualDur := mp4.FragmentDurationSecWithTimescales(media, trackTimescales)
		if actualDur <= 0 {
			actualDur = mp4.FragmentDurationSec(media)
		}
		if actualDur <= 0 && plSeg.DurSec > 0 {
			actualDur = plSeg.DurSec
		}

		seg := segmentReport{
			Index:            absIndex,
			ExpectedStartSec: expectedStart,
			MediaStartSec:    startSec,
			ExpectedDurSec:   expectedDur,
			ActualDurSec:     actualDur,
			Bytes:            len(media),
		}
		report.Segments = append(report.Segments, seg)

		if plSeg.DurSec > 0 {
			delta := actualDur - plSeg.DurSec
			if delta < 0 {
				delta = -delta
			}
			if delta > tolerance+0.05 {
				playlistOK = false
				report.Pass = false
				report.Issues = append(report.Issues, mp4.TimelineIssue{
					Check:    "playlist_extinf",
					Message:  fmt.Sprintf("segment %d #EXTINF=%.3f actualDur=%.3f", i, plSeg.DurSec, actualDur),
					DeltaSec: delta,
				})
			}
		}

		issues := mp4.ValidateSegmentTimeline(media, mp4.SegmentTimeline{
			Index:            absIndex,
			ExpectedStartSec: expectedStart,
			ExpectedDurSec:   expectedDur,
			MediaStartSec:    startSec,
			ActualDurSec:     actualDur,
			Bytes:            len(media),
		}, tolerance)
		issues = filterKeyframeAlignedDurationIssues(issues, actualDur, expectedDur, expectedStart, keyframeSeekTimes, tolerance)
		if prevSeg != nil {
			prevTL := mp4.SegmentTimeline{Index: prevSeg.Index, MediaStartSec: prevSeg.MediaStartSec, ActualDurSec: prevSeg.ActualDurSec}
			nextTL := mp4.SegmentTimeline{Index: absIndex, MediaStartSec: seg.MediaStartSec}
			issues = append(issues, mp4.ValidateContinuity(prevTL, nextTL, tolerance)...)
		}
		if len(issues) > 0 {
			report.Pass = false
			report.Issues = append(report.Issues, issues...)
		}
		prevSeg = &report.Segments[len(report.Segments)-1]
	}

	if !playlist.HasEndList {
		report.Pass = false
		playlistOK = false
		report.Issues = append(report.Issues, mp4.TimelineIssue{Check: "playlist", Message: "missing #EXT-X-ENDLIST"})
	}
	if playlist.TargetDur > 0 {
		for _, plSeg := range playlist.Segments {
			if plSeg.DurSec > playlist.TargetDur+0.001 {
				report.Pass = false
				playlistOK = false
				report.Issues = append(report.Issues, mp4.TimelineIssue{
					Check:   "target_duration",
					Message: fmt.Sprintf("#EXTINF %.3f exceeds #EXT-X-TARGETDURATION %.0f", plSeg.DurSec, playlist.TargetDur),
				})
			}
		}
	}

	report.PlaylistOK = playlistOK
	report.SegmentCount = len(report.Segments)
	return report, nil
}

func continuousOptions(file string, params goffmpeg.HLSSegmentParams, outDir string, starts, durations []float64, transcode bool, startIndex int, startSec float64) goffmpeg.HLSContinuousOptions {
	if transcode {
		durations = nil
	}
	if startIndex > 0 && startSec <= 0 && startIndex < len(starts) {
		startSec = starts[startIndex]
	}
	return goffmpeg.HLSContinuousOptions{
		Input:            goffmpeg.InputSource{URL: file, StreamType: probe.StreamFile},
		OutputDir:          outDir,
		StartIndex:         startIndex,
		StartSec:           startSec,
		SegmentDurations:   durations,
		SegmentSec:         goffmpeg.DefaultHLSSegmentDurationSec,
		FreshPlaylist:      startIndex == 0 && startSec <= 0,
		Decode:             params.Decode,
		Profile:            params.Profile,
		MaxHeight:          params.MaxHeight,
		Remux:              params.Remux,
		VideoCopy:          params.VideoCopy,
		GOP:                params.GOP,
	}
}

func printContinuousReport(r *continuousCheckReport) {
	status := "PASS"
	if !r.Pass {
		status = "FAIL"
	}
	fmt.Printf("%s  continuous file=%s mode=%s segments=%d init=%dB encode=%dms endlist=%t playlistOk=%t issues=%d audioIssues=%d\n",
		status, r.File, r.Mode, r.SegmentCount, r.InitBytes, r.TotalEncodeMs, r.HasEndList, r.PlaylistOK,
		len(r.Issues), len(r.AudioIssues))
	for _, seg := range r.Segments {
		fmt.Printf("  seg %d: start=%.3f (exp %.3f) dur=%.3f (exp %.3f) bytes=%d\n",
			seg.Index, seg.MediaStartSec, seg.ExpectedStartSec, seg.ActualDurSec, seg.ExpectedDurSec, seg.Bytes)
	}
	for _, issue := range r.Issues {
		fmt.Printf("  [%s] %s\n", issue.Check, issue.Message)
	}
	for _, issue := range r.AudioIssues {
		fmt.Printf("  [audio] %s\n", issue)
	}
}

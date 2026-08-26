package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func runRealworldCheck(args []string) int {
	fs := flag.NewFlagSet("realworld-check", flag.ExitOnError)
	files := fs.String("files", envOr("HLS_REALWORLD_FILES", ""), "comma-separated media paths to continuous-check (transcode)")
	mode := fs.String("mode", "transcode", "encode mode: remux, copy, transcode")
	tolerance := fs.Float64("tolerance", 0.15, "timeline tolerance seconds")
	debug := fs.Bool("debug", false, "stream ffmpeg stderr")
	_ = fs.Parse(args)

	paths := splitCSV(*files)
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "realworld-check: no files (use -files or HLS_REALWORLD_FILES)")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	svc, err := initFFmpeg(ctx, *debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg init: %v\n", err)
		return 1
	}

	failures := 0
	for _, file := range paths {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		label := filepathBase(file)
		info, err := svc.ProbeFile(ctx, file)
		if err != nil {
			fmt.Printf("FAIL %-40s probe: %v\n", label, err)
			failures++
			continue
		}
		params, err := paramsForMode(ctx, svc, file, info, *mode)
		if err != nil {
			fmt.Printf("FAIL %-40s %v\n", label, err)
			failures++
			continue
		}
		report, err := checkContinuousHLS(ctx, svc, file, params, "", *tolerance, 0, 0)
		if err != nil {
			fmt.Printf("FAIL %-40s %v\n", label, err)
			failures++
			continue
		}
		status := "PASS"
		if !report.Pass {
			status = "FAIL"
			failures++
		}
		fmt.Printf("%-4s %-40s segs=%d endlist=%t issues=%d encode=%dms\n",
			status, label, report.SegmentCount, report.HasEndList, len(report.Issues), report.TotalEncodeMs)
		for _, issue := range report.Issues {
			fmt.Printf("       ↳ [%s] %s\n", issue.Check, issue.Message)
		}
	}
	if failures > 0 {
		fmt.Printf("realworld-check: %d failure(s)\n", failures)
		return 1
	}
	fmt.Println("realworld-check: all passed")
	return 0
}

func filepathBase(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 && i+1 < len(path) {
		return path[i+1:]
	}
	return path
}

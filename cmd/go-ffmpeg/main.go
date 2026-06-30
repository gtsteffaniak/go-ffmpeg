// Command go-ffmpeg reports FFmpeg/FFprobe capability and compatibility on the host.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

const usageText = `go-ffmpeg — FFmpeg compatibility reporter

Detects installed FFmpeg/FFprobe binaries, probes encoders, decoders, filters,
protocols, platform GPU gates, and reports which library operations are supported.

Usage:
  go-ffmpeg [flags]

Examples:
  go-ffmpeg
  go-ffmpeg -color always
  go-ffmpeg -ffmpeg-path /usr/local/bin/ffmpeg
  go-ffmpeg -json
  go-ffmpeg -skip-hw-tests -o report.txt

Environment:
  GOFFMPEG_FFMPEG_PATH   Default -ffmpeg-path when flag is unset
  GOFFMPEG_FFPROBE_PATH  Default -ffprobe-path when flag is unset
  GOFFMPEG_SKIP_HW       When "1", same as -skip-hw-tests
`

func main() {
	os.Exit(run())
}

func run() int {
	ffmpegPath := flag.String("ffmpeg-path", envOr("GOFFMPEG_FFMPEG_PATH", ""), "path to ffmpeg binary or directory containing it")
	ffprobePath := flag.String("ffprobe-path", envOr("GOFFMPEG_FFPROBE_PATH", ""), "path to ffprobe binary or directory containing it")
	jsonOut := flag.Bool("json", false, "emit capability matrix as JSON")
	colorFlag := flag.String("color", "auto", "color output: auto, always, or never")
	skipHW := flag.Bool("skip-hw-tests", envOr("GOFFMPEG_SKIP_HW", "") == "1", "skip hardware encoder smoke tests")
	timeout := flag.Duration("timeout", ffmpeg.DefaultDetectTimeout, "capability detection timeout")
	output := flag.String("o", "", "write plain-text report to file (no ANSI colors)")
	showHelp := flag.Bool("help", false, "show help")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usageText)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showHelp {
		flag.Usage()
		return 0
	}

	color, err := parseColorFlag(*colorFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "go-ffmpeg: %v\n", err)
		return 2
	}

	ctx := context.Background()
	cfg := ffmpeg.Config{
		FFmpegPath:    *ffmpegPath,
		FFprobePath:   *ffprobePath,
		DetectTimeout: *timeout,
		SkipHWTests:   *skipHW,
		Logger:        ffmpeg.NopLogger(),
	}
	svc, err := ffmpeg.New(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "go-ffmpeg: detection failed: %v\n", err)
		return 1
	}

	caps := svc.Capabilities()
	if caps == nil {
		fmt.Fprintln(os.Stderr, "go-ffmpeg: no capabilities detected")
		return 1
	}

	if *jsonOut {
		data, err := caps.JSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "go-ffmpeg: json encode failed: %v\n", err)
			return 1
		}
		report := string(data) + "\n"
		if *output != "" {
			return writeReport(*output, report)
		}
		fmt.Print(report)
		return 0
	}

	if *output != "" {
		var b strings.Builder
		if err := caps.ReportWithOptions(&b, capabilities.ReportOptions{Color: boolPtr(false)}); err != nil {
			fmt.Fprintf(os.Stderr, "go-ffmpeg: report failed: %v\n", err)
			return 1
		}
		return writeReport(*output, b.String())
	}

	opts := capabilities.ReportOptions{Color: color}
	if err := caps.ReportWithOptions(os.Stdout, opts); err != nil {
		fmt.Fprintf(os.Stderr, "go-ffmpeg: report failed: %v\n", err)
		return 1
	}
	return 0
}

func parseColorFlag(v string) (*bool, error) {
	switch v {
	case "auto", "":
		return nil, nil
	case "always", "on", "true", "1":
		return boolPtr(true), nil
	case "never", "off", "false", "0", "none":
		return boolPtr(false), nil
	default:
		return nil, fmt.Errorf("unknown -color value %q (use auto, always, or never)", v)
	}
}

func boolPtr(v bool) *bool { return &v }

func writeReport(path, content string) int {
	if content != "" && content[len(content)-1] != '\n' {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "go-ffmpeg: write output: %v\n", err)
		return 1
	}
	return 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "hls-check":
		os.Exit(runHLSCheck(os.Args[2:]))
	case "matrix":
		os.Exit(runMatrix(os.Args[2:]))
	case "run":
		os.Exit(runFullTest(os.Args[2:]))
	case "generate-fixtures":
		os.Exit(runGenerateFixtures(os.Args[2:]))
	case "serve-report":
		os.Exit(runReportServe(os.Args[2:]))
	case "serve-hls":
		os.Exit(runServeHLS(os.Args[2:]))
	case "playback-test":
		os.Exit(runPlaybackTest(os.Args[2:]))
	case "hw-check":
		os.Exit(runHWCheck(os.Args[2:]))
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `test-ffmpeg — fast HLS timeline validation harness

Usage:
  test-ffmpeg generate-fixtures [flags]
  test-ffmpeg run [flags]         Full pipeline: fixtures + benchmarks + report
  test-ffmpeg serve-report [flags]
  test-ffmpeg hls-check [flags]
  test-ffmpeg matrix [flags]
  test-ffmpeg serve-hls [flags]
  test-ffmpeg playback-test [flags]

Run "test-ffmpeg hls-check -h" or "test-ffmpeg matrix -h" for flags.
`)
}

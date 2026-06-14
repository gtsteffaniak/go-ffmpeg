package main_test

import (
	"flag"
	"os"
	"testing"
)

func TestCLIHelp(t *testing.T) {
	if os.Getenv("GOFFMPEG_CLI_TEST") != "1" {
		t.Skip("set GOFFMPEG_CLI_TEST=1 to run CLI subprocess tests")
	}
}

func TestFlagDefaults(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	ffmpegPath := fs.String("ffmpeg-path", "", "")
	skipHW := fs.Bool("skip-hw-tests", false, "")
	_ = fs.Parse([]string{})
	if *ffmpegPath != "" {
		t.Fatalf("expected empty default ffmpeg path")
	}
	if *skipHW {
		t.Fatal("expected skip-hw false by default")
	}
}

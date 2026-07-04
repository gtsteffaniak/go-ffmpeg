package ops

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

func TestFilterMatroskaSeekNoise(t *testing.T) {
	t.Parallel()
	in := []byte("[in#0/matroska,webm @ 0x1] 0x00 at pos 8408958 invalid as first byte of an EBML number\n[eac3 @ 0x2] exponent -2 is out-of-range\n")
	out := filterMatroskaSeekNoise(in)
	if strings.Contains(string(out), "EBML number") {
		t.Fatalf("expected EBML line filtered, got %q", out)
	}
	if !strings.Contains(string(out), "eac3") {
		t.Fatalf("expected real error preserved, got %q", out)
	}
}

func TestVTDecodeStderrMonitorFiltersMatroskaNoise(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := newVTDecodeStderrMonitor(&buf)
	msg := "[in#0/matroska,webm @ 0x1] 0x00 at pos 8408958 invalid as first byte of an EBML number\n"
	if _, err := m.Write([]byte(msg)); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected matroska noise suppressed, got %q", buf.String())
	}
}

func TestBuildHLSContinuousArgsLogLevel(t *testing.T) {
	t.Parallel()
	caps := &capabilities.Capabilities{}
	opts := HLSContinuousOptions{
		Input:      InputSource{URL: "/media/video.mkv"},
		OutputDir:  "/cache/job",
		SegmentSec: 4,
		StartSec:   3704,
		VideoCopy:  true,
	}

	quiet, err := buildHLSContinuousArgs(&ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg"}, caps, opts)
	if err != nil {
		t.Fatalf("buildHLSContinuousArgs: %v", err)
	}
	if quiet[2] != "-loglevel" || quiet[3] != "error" {
		t.Fatalf("expected -loglevel error for non-verbose runner, got %v", quiet[:6])
	}

	verbose, err := buildHLSContinuousArgs(&ffexec.Runner{FFmpegPath: "/usr/bin/ffmpeg", VerboseFFmpeg: true}, caps, opts)
	if err != nil {
		t.Fatalf("buildHLSContinuousArgs verbose: %v", err)
	}
	if verbose[2] != "-loglevel" || verbose[3] != "info" {
		t.Fatalf("expected -loglevel info for verbose runner, got %v", verbose[:6])
	}
}

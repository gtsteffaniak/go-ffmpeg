package main

import (
	"os"
	"strings"
	"testing"
)

func TestDefaultSampleVideo(t *testing.T) {
	path := defaultSampleVideo()
	if !strings.HasSuffix(path, defaultSampleFilename) {
		t.Fatalf("path=%q want suffix %q", path, defaultSampleFilename)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default sample missing at %q: %v", path, err)
	}
}

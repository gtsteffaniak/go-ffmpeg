package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFFmpegM3U8RejectsInvalidEXTINF(t *testing.T) {
	t.Parallel()
	path := writePlaylist(t, "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:invalid,\nseg/00000.m4s\n")
	_, err := parseFFmpegM3U8(path)
	if err == nil || !strings.Contains(err.Error(), "#EXTINF") {
		t.Fatalf("expected invalid #EXTINF error, got %v", err)
	}
}

func TestParseFFmpegM3U8RejectsInvalidTargetDuration(t *testing.T) {
	t.Parallel()
	path := writePlaylist(t, "#EXTM3U\n#EXT-X-TARGETDURATION:nope\n#EXTINF:4.0,\nseg/00000.m4s\n")
	_, err := parseFFmpegM3U8(path)
	if err == nil || !strings.Contains(err.Error(), "#EXT-X-TARGETDURATION") {
		t.Fatalf("expected invalid target duration error, got %v", err)
	}
}

func TestParseFFmpegM3U8RejectsSegmentWithoutEXTINF(t *testing.T) {
	t.Parallel()
	path := writePlaylist(t, "#EXTM3U\n#EXT-X-TARGETDURATION:4\nseg/00000.m4s\n")
	_, err := parseFFmpegM3U8(path)
	if err == nil || !strings.Contains(err.Error(), "missing a preceding #EXTINF") {
		t.Fatalf("expected missing #EXTINF error, got %v", err)
	}
}

func TestParseFFmpegM3U8ValidPlaylist(t *testing.T) {
	t.Parallel()
	path := writePlaylist(t, "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4.0,\nseg/00000.m4s\n#EXT-X-ENDLIST\n")
	pl, err := parseFFmpegM3U8(path)
	if err != nil {
		t.Fatal(err)
	}
	if !pl.HasEndList || pl.TargetDur != 4 || len(pl.Segments) != 1 || pl.Segments[0].DurSec != 4 {
		t.Fatalf("playlist = %+v", pl)
	}
}

func writePlaylist(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg.m3u8")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

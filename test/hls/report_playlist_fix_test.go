package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixReportContinuousPlaylists(t *testing.T) {
	t.Parallel()
	reportDir := t.TempDir()
	mediaDir := filepath.Join(reportDir, "media", "fixture", "transcode_software_continuous_full")
	segDir := filepath.Join(mediaDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "00000.m4s"), []byte("segment-bytes-long-enough"), 0o644); err != nil {
		t.Fatal(err)
	}
	playlist := filepath.Join(mediaDir, "ffmpeg.m3u8")
	if err := os.WriteFile(playlist, []byte("#EXTM3U\n#EXTINF:4.0,\n00000.m4s\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := fixReportContinuousPlaylists(reportDir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("fixed=%d want 1", n)
	}
	out, err := os.ReadFile(playlist)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "#EXTM3U\n#EXTINF:4.0,\nseg/00000.m4s\n" {
		t.Fatalf("playlist = %q", string(out))
	}
}

func TestFinalizeContinuousOutputDir(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	segDir := filepath.Join(outDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "00001.m4s"), []byte("segment-bytes-long-enough"), 0o644); err != nil {
		t.Fatal(err)
	}
	playlist := filepath.Join(outDir, "ffmpeg.m3u8")
	if err := os.WriteFile(playlist, []byte("#EXTM3U\n#EXTINF:4.0,\n00001.m4s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := finalizeContinuousOutputDir(outDir); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(playlist)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "#EXTM3U\n#EXTINF:4.0,\nseg/00001.m4s\n" {
		t.Fatalf("playlist = %q", string(out))
	}
}

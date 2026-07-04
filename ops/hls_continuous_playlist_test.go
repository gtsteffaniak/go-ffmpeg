package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixContinuousPlaylistSegmentURIs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	segDir := filepath.Join(dir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "00000.m4s"), []byte("segment-bytes-long-enough"), 0o644); err != nil {
		t.Fatal(err)
	}
	playlist := filepath.Join(dir, "ffmpeg.m3u8")
	contents := "#EXTM3U\n#EXT-X-MAP:URI=\"init.m4s\"\n#EXTINF:4.0,\n00000.m4s\n"
	if err := os.WriteFile(playlist, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := FixContinuousPlaylistSegmentURIs(dir, playlist); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(playlist)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "#EXTM3U\n#EXT-X-MAP:URI=\"init.m4s\"\n#EXTINF:4.0,\nseg/00000.m4s\n" {
		t.Fatalf("playlist = %q", string(out))
	}
}

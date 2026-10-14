package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeContinuousPlaylist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	segDir := filepath.Join(dir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "00000.m4s"), []byte("segment-bytes-long-enough-for-serve-check"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg.m3u8"), []byte("#EXTM3U\n#EXT-X-MAP:URI=\"init.m4s\"\n#EXTINF:4.0,\n00000.m4s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NormalizeContinuousPlaylist(dir); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(dir, "ffmpeg.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "#EXTM3U\n#EXT-X-MAP:URI=\"init.m4s\"\n#EXTINF:4.0,\nseg/00000.m4s\n" {
		t.Fatalf("playlist = %q", string(out))
	}
}

func TestContinuousSegmentServeReadyRequiresReadyMarker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	segDir := filepath.Join(dir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(segDir, "00000.m4s")
	payload := make([]byte, 9000)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if ContinuousSegmentServeReady(dir, "00000.m4s") {
		t.Fatal("expected segment without .ready marker to be not serve-ready")
	}
	if err := markContinuousSegmentReady(path); err != nil {
		t.Fatal(err)
	}
	if !ContinuousSegmentServeReady(dir, "00000.m4s") {
		t.Fatal("expected aligned segment to be serve-ready")
	}
}

func TestResolveContinuousSegmentPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveContinuousSegmentPath(dir, "00001.m4s")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "seg", "00001.m4s")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

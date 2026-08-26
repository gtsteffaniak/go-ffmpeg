package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseContinuousPlaylistRejectsInvalidEXTINF(t *testing.T) {
	t.Parallel()
	path := writeOpsPlaylist(t, "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:invalid,\nseg/00000.m4s\n")
	_, err := parseContinuousPlaylist(path)
	if err == nil || !strings.Contains(err.Error(), "#EXTINF") {
		t.Fatalf("expected invalid #EXTINF error, got %v", err)
	}
}

func TestParseContinuousPlaylistRejectsSegmentWithoutEXTINF(t *testing.T) {
	t.Parallel()
	path := writeOpsPlaylist(t, "#EXTM3U\n#EXT-X-TARGETDURATION:4\nseg/00000.m4s\n")
	_, err := parseContinuousPlaylist(path)
	if err == nil || !strings.Contains(err.Error(), "missing a preceding #EXTINF") {
		t.Fatalf("expected missing #EXTINF error, got %v", err)
	}
}

func TestStderrTailBoundsCapture(t *testing.T) {
	t.Parallel()
	tail := newStderrTail(16)
	if _, err := tail.Write([]byte("abcdefghijklmnopqrstuvwxyz")); err != nil {
		t.Fatal(err)
	}
	if got := tail.String(); got != "klmnopqrstuvwxyz" {
		t.Fatalf("tail = %q, want last 16 bytes", got)
	}
}

func writeOpsPlaylist(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg.m3u8")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

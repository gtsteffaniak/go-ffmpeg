package ops

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVODAbsLocalInputURLPreservesRemoteSchemes(t *testing.T) {
	t.Parallel()
	for _, url := range []string{"", "http://host/video.mkv", "rtsp://cam/stream"} {
		if vodAbsLocalInputURL(url) != url {
			t.Fatalf("remote URL changed: %q", url)
		}
	}
}

func TestVODAbsLocalInputURLAbsolutesLocalPaths(t *testing.T) {
	t.Parallel()
	got := vodAbsLocalInputURL("clip.mkv")
	if strings.Contains(got, "://") {
		t.Fatalf("local path became URL: %q", got)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("expected absolute path, got %q", got)
	}
}

package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareContinuousOutputLayoutClearsNewGeneration(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	segDir := filepath.Join(outDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "00042.m4s"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, continuousServePlaylistName), []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := HLSContinuousOptions{StartIndex: 10}
	if err := prepareContinuousOutputLayout(outDir, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(segDir, "00042.m4s")); !os.IsNotExist(err) {
		t.Fatalf("stale seg should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, continuousServePlaylistName)); !os.IsNotExist(err) {
		t.Fatalf("stale index should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "raw")); err != nil {
		t.Fatalf("raw dir: %v", err)
	}
}

func TestPrepareContinuousOutputLayoutKeepsAppendListResume(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	segDir := filepath.Join(outDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(segDir, "00001.m4s")
	if err := os.WriteFile(stale, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := HLSContinuousOptions{StartIndex: 0, FreshPlaylist: false}
	if err := prepareContinuousOutputLayout(outDir, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("append_list resume should keep prior seg/: %v", err)
	}
}

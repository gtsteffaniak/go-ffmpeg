package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHLSVODSegmentReadyRejectsTmpSibling(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "seg", "00001.m4s")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, minHLSSegmentMediaBytes+1)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+hlsVODSegmentTmpSuffix, []byte("writing"), 0o644); err != nil {
		t.Fatal(err)
	}
	ready, err := HLSVODSegmentReady(path)
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatal("expected not ready while .tmp sibling exists")
	}
}

package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

// errAfterFirstWrite simulates an HTTP client that disconnects mid-response.
type errAfterFirstWrite struct {
	writes int
	err    error
}

func (w *errAfterFirstWrite) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == 1 {
		return len(p), nil
	}
	return 0, w.err
}

func TestRunFrameCaptureToWriterReturnsOnDestinationError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fakeFFmpeg := filepath.Join(dir, "fake-ffmpeg")
	script := "#!/bin/sh\nwhile :; do dd if=/dev/zero bs=65536 count=1 2>/dev/null || exit 1; done\n"
	if err := os.WriteFile(fakeFFmpeg, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runner := &ffexec.Runner{FFmpegPath: fakeFFmpeg}
	disconnectErr := errors.New("client disconnected")
	dest := &errAfterFirstWrite{err: disconnectErr}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := runFrameCaptureToWriter(ctx, runner, nil, dest)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected destination write error")
	}
	if !errors.Is(err, disconnectErr) {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("runFrameCaptureToWriter hung for %v", elapsed)
	}
}

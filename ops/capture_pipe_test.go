package ops

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func buildStreamingFakeFFmpeg(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.go")
	const src = `package main

import "os"

func main() {
	buf := make([]byte, 65536)
	for {
		if _, err := os.Stdout.Write(buf); err != nil {
			os.Exit(1)
		}
	}
}
`
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "fake-ffmpeg")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, srcPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake ffmpeg: %v\n%s", err, output)
	}
	return out
}

func TestRunFrameCaptureToWriterReturnsOnDestinationError(t *testing.T) {
	t.Parallel()

	runner := &ffexec.Runner{FFmpegPath: buildStreamingFakeFFmpeg(t)}
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

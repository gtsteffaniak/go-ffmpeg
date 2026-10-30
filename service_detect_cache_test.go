package ffmpeg

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// newDetectCountingService returns a service whose detection has not run yet.
func newDetectCountingService(t *testing.T) *Service {
	t.Helper()
	off := false
	svc, err := New(context.Background(), Config{
		FFmpegPath:   testFFmpegPath(t),
		FFprobePath:  testFFprobePath(t),
		DetectOnInit: &off,
		SkipHWTests:  true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

func TestEnsureDetectedRunsDetectionOnce(t *testing.T) {
	svc := newDetectCountingService(t)
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		if err := svc.ensureDetected(ctx); err != nil {
			t.Fatalf("ensureDetected call %d: %v", i, err)
		}
	}
	svc.mu.RLock()
	runs, caps := svc.detectRuns, svc.caps
	svc.mu.RUnlock()
	if runs != 1 {
		t.Fatalf("detection ran %d times after 50 calls, want 1", runs)
	}
	if caps == nil {
		t.Fatal("capabilities not cached after detection")
	}
}

func TestReloadIsTheOnlyRedetection(t *testing.T) {
	svc := newDetectCountingService(t)
	ctx := context.Background()
	if err := svc.ensureDetected(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	svc.mu.RLock()
	runs := svc.detectRuns
	svc.mu.RUnlock()
	if runs != 2 {
		t.Fatalf("detection runs = %d after ensure+Reload, want 2", runs)
	}
}

func TestFailedDetectionIsCachedNotRetried(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake ffmpeg script needs a POSIX shell")
	}
	svc := newDetectCountingService(t)
	fake := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Swap the binary after New so detection fails on the broken path.
	svc.runner.FFmpegPath = fake

	ctx := context.Background()
	first := svc.ensureDetected(ctx)
	if first == nil {
		t.Skip("detection did not fail for a broken binary on this platform")
	}
	for i := 0; i < 20; i++ {
		if err := svc.ensureDetected(ctx); err != first {
			t.Fatalf("call %d returned %v, want the cached first error %v", i, err, first)
		}
	}
	svc.mu.RLock()
	runs := svc.detectRuns
	svc.mu.RUnlock()
	if runs != 1 {
		t.Fatalf("failed detection re-ran %d times, want 1", runs)
	}
}

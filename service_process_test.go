package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"testing"
)

func testFFmpegPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("GOFFMPEG_FFMPEG_PATH"); p != "" {
		return p
	}
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatalf("ffmpeg is required: %v", err)
	}
	return p
}

func testFFprobePath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("GOFFMPEG_FFPROBE_PATH"); p != "" {
		return p
	}
	p, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatalf("ffprobe is required: %v", err)
	}
	return p
}

func TestEnsureDetectedConcurrent(t *testing.T) {
	ctx := context.Background()
	svc, err := New(ctx, Config{
		FFmpegPath:   testFFmpegPath(t),
		FFprobePath:  testFFprobePath(t),
		DetectOnInit: boolPtr(false),
		SkipHWTests:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	const workers = 16
	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = svc.ensureDetected(ctx)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}
	caps := svc.Capabilities()
	if caps == nil || caps.FFmpegVersion == "" {
		t.Fatal("expected capabilities after concurrent lazy detection")
	}
}

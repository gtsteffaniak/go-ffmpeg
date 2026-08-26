package ffmpeg

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestWrapOpClassifiesStderr(t *testing.T) {
	t.Parallel()
	err := wrapOpWithStderr("Transcode", ErrEncodeFailed, errors.New("exit status 1"), "Conversion failed!")
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected OperationError, got %T", err)
	}
	if opErr.Kind != encode.FailureEncode {
		t.Fatalf("kind = %q, want %q", opErr.Kind, encode.FailureEncode)
	}
	if opErr.Stderr != "Conversion failed!" {
		t.Fatalf("stderr = %q", opErr.Stderr)
	}
}

func TestServiceMaxConcurrentBlocksSecondAcquire(t *testing.T) {
	cfg := (&Config{MaxConcurrent: 1}).withDefaults()
	s := &Service{
		cfg:       cfg,
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
	}

	ctx := context.Background()
	if err := s.Acquire(ctx); err != nil {
		t.Fatal(err)
	}

	acquired := make(chan error, 1)
	go func() {
		acquired <- s.Acquire(ctx)
	}()

	select {
	case err := <-acquired:
		t.Fatalf("expected second acquire to block, got err=%v", err)
	case <-time.After(75 * time.Millisecond):
	}

	s.Release()
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatalf("second acquire: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second acquire did not complete")
	}
}

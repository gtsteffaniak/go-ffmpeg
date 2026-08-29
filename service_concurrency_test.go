package ffmpeg

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/concurrency"
)

func TestMixedProbeDecodeConcurrency(t *testing.T) {
	cfg := (&Config{
		Concurrency: Concurrency{
			MaxProbe:     8,
			MaxDecode:    1,
			MaxEncode:    1,
			MaxLargeFile: concurrency.IntPtr(0),
		},
	}).withDefaults()
	s := &Service{
		cfg:     cfg,
		limiter: concurrency.NewLimiter(cfg.Concurrency),
	}

	ctx := context.Background()
	if err := s.Acquire(ctx, SlotDecode); err != nil {
		t.Fatal(err)
	}
	defer s.Release(SlotDecode)

	const probes = 4
	var wg sync.WaitGroup
	errCh := make(chan error, probes)
	for i := 0; i < probes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Acquire(ctx, SlotProbe); err != nil {
				errCh <- err
				return
			}
			defer s.Release(SlotProbe)
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("probe acquires blocked while decode slot held")
	}

	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("probe acquire: %v", err)
		}
	}
}

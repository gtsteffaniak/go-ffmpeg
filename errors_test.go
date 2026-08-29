package ffmpeg

import (
	"context"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/concurrency"
)

func TestServiceMaxConcurrentBlocksSecondAcquire(t *testing.T) {
	cfg := (&Config{MaxConcurrent: 1}).withDefaults()
	s := &Service{
		cfg:     cfg,
		limiter: concurrency.NewLimiter(cfg.Concurrency),
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

func TestResolveConcurrencyLegacyMaxConcurrent(t *testing.T) {
	cfg := (&Config{MaxConcurrent: 2}).withDefaults()
	if cfg.Concurrency.MaxDecode != 2 || cfg.Concurrency.MaxEncode != 2 {
		t.Fatalf("legacy decode/encode = %d/%d", cfg.Concurrency.MaxDecode, cfg.Concurrency.MaxEncode)
	}
	if cfg.Concurrency.MaxProbe < 16 {
		t.Fatalf("legacy MaxProbe = %d, want >= 16", cfg.Concurrency.MaxProbe)
	}
}

func TestResolveConcurrencyExplicitTiers(t *testing.T) {
	cfg := (&Config{
		Concurrency: Concurrency{
			MaxProbe:  8,
			MaxDecode: 3,
			MaxEncode: 1,
			GlobalMax: 10,
		},
	}).withDefaults()
	if cfg.Concurrency.MaxProbe != 8 || cfg.Concurrency.MaxDecode != 3 || cfg.Concurrency.MaxEncode != 1 {
		t.Fatalf("explicit tiers overwritten: %+v", cfg.Concurrency)
	}
	if cfg.Concurrency.GlobalMax != 10 {
		t.Fatalf("GlobalMax = %d, want 10", cfg.Concurrency.GlobalMax)
	}
}

func TestResolveConcurrencyLargeFileSettings(t *testing.T) {
	threshold := int64(1 << 30)
	limit := 1
	cfg := (&Config{
		Concurrency: Concurrency{
			MaxLargeFile:            &limit,
			LargeFileThresholdBytes: threshold,
		},
	}).withDefaults()
	if cfg.Concurrency.LargeFileThresholdBytes != threshold {
		t.Fatalf("LargeFileThresholdBytes = %d, want %d", cfg.Concurrency.LargeFileThresholdBytes, threshold)
	}
	if cfg.Concurrency.MaxLargeFile == nil || *cfg.Concurrency.MaxLargeFile != limit {
		t.Fatalf("MaxLargeFile = %v, want %d", cfg.Concurrency.MaxLargeFile, limit)
	}
}

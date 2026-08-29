package concurrency

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLegacyFromMaxConcurrent(t *testing.T) {
	t.Parallel()
	cfg := LegacyFromMaxConcurrent(2)
	if cfg.MaxDecode != 2 || cfg.MaxEncode != 2 {
		t.Fatalf("decode/encode = %d/%d, want 2/2", cfg.MaxDecode, cfg.MaxEncode)
	}
	if cfg.MaxProbe < 16 {
		t.Fatalf("MaxProbe = %d, want >= 16", cfg.MaxProbe)
	}
}

func TestLimiterAcquireBlocksSameTier(t *testing.T) {
	t.Parallel()
	l := NewLimiter(Config{MaxDecode: 1, MaxProbe: 4, MaxEncode: 1, MaxLargeFile: 0})
	ctx := context.Background()

	if err := l.Acquire(ctx, SlotDecode); err != nil {
		t.Fatal(err)
	}

	acquired := make(chan error, 1)
	go func() {
		acquired <- l.Acquire(ctx, SlotDecode)
	}()

	select {
	case err := <-acquired:
		t.Fatalf("expected second decode acquire to block, got err=%v", err)
	case <-time.After(50 * time.Millisecond):
	}

	l.Release(SlotDecode)

	select {
	case err := <-acquired:
		if err != nil {
			t.Fatalf("second acquire: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second acquire did not complete")
	}
}

func TestLimiterProbeDoesNotBlockDecode(t *testing.T) {
	t.Parallel()
	l := NewLimiter(Config{MaxDecode: 1, MaxProbe: 4, MaxEncode: 1, MaxLargeFile: 0})
	ctx := context.Background()

	if err := l.Acquire(ctx, SlotDecode); err != nil {
		t.Fatal(err)
	}
	defer l.Release(SlotDecode)

	done := make(chan error, 1)
	go func() {
		done <- l.Acquire(ctx, SlotProbe)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("probe acquire: %v", err)
		}
		l.Release(SlotProbe)
	case <-time.After(time.Second):
		t.Fatal("probe acquire blocked while decode slot held")
	}
}

func TestLimiterGlobalMax(t *testing.T) {
	t.Parallel()
	l := NewLimiter(Config{MaxProbe: 8, MaxDecode: 8, MaxEncode: 8, GlobalMax: 1, MaxLargeFile: 0})
	ctx := context.Background()

	if err := l.Acquire(ctx, SlotProbe); err != nil {
		t.Fatal(err)
	}

	blocked := make(chan error, 1)
	go func() {
		blocked <- l.Acquire(ctx, SlotDecode)
	}()

	select {
	case err := <-blocked:
		t.Fatalf("expected global cap to block, got err=%v", err)
	case <-time.After(50 * time.Millisecond):
	}

	l.Release(SlotProbe)

	select {
	case err := <-blocked:
		if err != nil {
			t.Fatalf("decode acquire: %v", err)
		}
		l.Release(SlotDecode)
	case <-time.After(time.Second):
		t.Fatal("decode acquire did not complete after probe release")
	}
}

func TestNeedsLargeFileSlot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	small := filepath.Join(dir, "small.bin")
	if err := os.WriteFile(small, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	threshold := int64(500 << 20)
	if needsLargeFileSlot(small, threshold) {
		t.Fatal("small file should not need large slot")
	}
	if needsLargeFileSlot("", threshold) {
		t.Fatal("empty path should not need large slot")
	}
}

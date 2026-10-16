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

func TestWithDefaultsPreservesExplicitLargeFileDisable(t *testing.T) {
	t.Parallel()
	cfg := Config{MaxLargeFile: IntPtr(0)}.WithDefaults()
	if cfg.MaxLargeFile == nil || *cfg.MaxLargeFile != 0 {
		t.Fatalf("MaxLargeFile = %v, want explicit 0", cfg.MaxLargeFile)
	}
}

func TestNewLimiterRespectsExplicitLargeFileDisable(t *testing.T) {
	t.Parallel()
	l := NewLimiter(Config{
		MaxProbe:     4,
		MaxDecode:    2,
		MaxEncode:    1,
		MaxLargeFile: IntPtr(0),
	})
	if l.largeOn {
		t.Fatal("large-file tier should be disabled when MaxLargeFile is 0")
	}

	dir := t.TempDir()
	large := filepath.Join(dir, "large.bin")
	if err := os.WriteFile(large, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	threshold := int64(512)
	l = NewLimiter(Config{
		MaxProbe:                4,
		MaxDecode:               1,
		MaxEncode:               1,
		MaxLargeFile:            IntPtr(1),
		LargeFileThresholdBytes: threshold,
	})
	if err := os.WriteFile(large, make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}

	lease1, err := l.AcquireLease(ctx, SlotDecode, large)
	if err != nil {
		t.Fatal(err)
	}
	defer lease1.Release()

	blocked := make(chan error, 1)
	go func() {
		_, err := l.AcquireLease(ctx, SlotDecode, large)
		blocked <- err
	}()

	select {
	case err := <-blocked:
		t.Fatalf("expected large-file cap to block, got err=%v", err)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestLimiterAcquireBlocksSameTier(t *testing.T) {
	t.Parallel()
	l := NewLimiter(Config{MaxDecode: 1, MaxProbe: 4, MaxEncode: 1, MaxLargeFile: IntPtr(0)})
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
	l := NewLimiter(Config{MaxDecode: 1, MaxProbe: 4, MaxEncode: 1, MaxLargeFile: IntPtr(0)})
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
	l := NewLimiter(Config{MaxProbe: 8, MaxDecode: 8, MaxEncode: 8, GlobalMax: 1, MaxLargeFile: IntPtr(0)})
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

func largeTestFile(t *testing.T, dir, name string, threshold int64) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, make([]byte, threshold+1), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func limiterTestConfig(threshold int64) Config {
	return Config{
		MaxProbe:                8,
		MaxDecode:               2,
		MaxEncode:               2,
		GlobalMax:               4,
		MaxLargeFile:            IntPtr(2),
		LargeFileThresholdBytes: threshold,
	}
}

func heldSlots(ch chan struct{}) int {
	if ch == nil {
		return 0
	}
	return len(ch)
}

func TestAcquireLeaseCancelWaitingEncodeReleasesPartial(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	threshold := int64(1024)
	large := largeTestFile(t, dir, "big.mkv", threshold)
	cfg := limiterTestConfig(threshold)
	cfg.MaxEncode = 1
	l := NewLimiter(cfg)

	ctx := context.Background()

	// Hold the only encode tier slot on this large file.
	encodeLease, err := l.AcquireLease(ctx, SlotEncode, large)
	if err != nil {
		t.Fatal(err)
	}

	waitCtx, cancel := context.WithCancel(ctx)
	errCh := make(chan error, 1)
	go func() {
		_, err := l.AcquireLease(waitCtx, SlotEncode, large)
		errCh <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled acquire did not return")
	}

	// Waiter must not leak global/large slots; only the first encode lease still holds resources.
	if n := heldSlots(l.large); n != 1 {
		t.Fatalf("large channel holds %d slots after cancel, want 1 (first lease only)", n)
	}
	if n := heldSlots(l.global); n != 1 {
		t.Fatalf("global channel holds %d slots after cancel, want 1 (first lease only)", n)
	}

	encodeLease.Release()

	if heldSlots(l.encode) != 0 || heldSlots(l.large) != 0 || heldSlots(l.global) != 0 {
		t.Fatalf("after full release: encode=%d large=%d global=%d, want 0/0/0",
			heldSlots(l.encode), heldSlots(l.large), heldSlots(l.global))
	}

	start := time.Now()
	probeLease, err := l.AcquireLease(ctx, SlotProbe, large)
	if err != nil {
		t.Fatal(err)
	}
	probeLease.Release()
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Fatalf("large-file probe acquire took %v, want <= 10ms", elapsed)
	}
}

func TestProbeExemptFromLargeFileWithTwoContinuousEncodes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	threshold := int64(1024)
	large1 := largeTestFile(t, dir, "a.mkv", threshold)
	large2 := largeTestFile(t, dir, "b.mkv", threshold)
	large3 := largeTestFile(t, dir, "c.mkv", threshold)
	l := NewLimiter(limiterTestConfig(threshold))
	ctx := context.Background()

	lease1, err := l.AcquireLease(ctx, SlotEncode, large1)
	if err != nil {
		t.Fatal(err)
	}
	defer lease1.Release()

	lease2, err := l.AcquireLease(ctx, SlotEncode, large2)
	if err != nil {
		t.Fatal(err)
	}
	defer lease2.Release()

	type result struct {
		lease *Lease
		err   error
	}
	done := make(chan result, 1)
	go func() {
		lease, err := l.AcquireLease(ctx, SlotProbe, large3)
		done <- result{lease, err}
	}()

	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("probe on third large file: %v", res.err)
		}
		res.lease.Release()
	case <-time.After(time.Second):
		t.Fatal("probe blocked while two large-file encodes held large slots")
	}
}

func TestLeaseDoubleReleaseDoesNotBlock(t *testing.T) {
	t.Parallel()
	l := NewLimiter(Config{MaxProbe: 2, MaxDecode: 1, MaxEncode: 1, MaxLargeFile: IntPtr(0)})
	ctx := context.Background()
	lease, err := l.AcquireLease(ctx, SlotProbe, "")
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		lease.Release()
		lease.Release()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("double Release blocked")
	}
}

func TestLimiterReleaseWithoutAcquireCountsMisrelease(t *testing.T) {
	t.Parallel()
	l := NewLimiter(Config{MaxProbe: 2, MaxDecode: 1, MaxEncode: 1, MaxLargeFile: IntPtr(0)})
	l.Release(SlotProbe)
	if got := l.MisreleaseCount.Load(); got != 1 {
		t.Fatalf("MisreleaseCount = %d, want 1", got)
	}
	l.Release(SlotProbe)
	if got := l.MisreleaseCount.Load(); got != 2 {
		t.Fatalf("MisreleaseCount = %d, want 2", got)
	}
}

func TestProbeDoesNotTakeLargeFileSlot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	threshold := int64(512)
	large := largeTestFile(t, dir, "big.bin", threshold)
	l := NewLimiter(Config{
		MaxProbe:                4,
		MaxDecode:               1,
		MaxEncode:               1,
		MaxLargeFile:            IntPtr(1),
		LargeFileThresholdBytes: threshold,
	})
	ctx := context.Background()

	probeLease, err := l.AcquireLease(ctx, SlotProbe, large)
	if err != nil {
		t.Fatal(err)
	}
	defer probeLease.Release()

	if n := heldSlots(l.large); n != 0 {
		t.Fatalf("probe took %d large-file slots, want 0", n)
	}
}

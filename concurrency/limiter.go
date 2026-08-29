package concurrency

import (
	"context"
	"os"
)

// SlotClass identifies which concurrency tier an operation uses.
type SlotClass int

const (
	SlotProbe SlotClass = iota
	SlotDecode
	SlotEncode
	slotLargeFile
)

// Config holds per-tier subprocess limits. Zero values are filled by WithDefaults.
type Config struct {
	// MaxProbe limits concurrent ffprobe-style work (metadata, duration, dimensions).
	MaxProbe int

	// MaxDecode limits short ffmpeg jobs: previews, screenshots, remux, stream copy.
	MaxDecode int

	// MaxEncode limits heavy transcode / segment encode jobs.
	MaxEncode int

	// GlobalMax is an optional cap on concurrent subprocesses across all tiers.
	// Zero disables the global cap.
	GlobalMax int

	// LargeFileThresholdBytes triggers an extra large-file slot when the input exceeds this size.
	LargeFileThresholdBytes int64

	// MaxLargeFile limits concurrent operations on inputs above LargeFileThresholdBytes.
	// Nil uses the default limit (2). A non-nil pointer to zero disables the tier.
	MaxLargeFile *int
}

const defaultLargeFileThreshold = 500 << 20 // 500 MiB

// IntPtr returns a pointer to n (helper for explicit MaxLargeFile values).
func IntPtr(n int) *int {
	return &n
}

// WithDefaults returns cfg with unset fields populated.
func (c Config) WithDefaults() Config {
	out := c
	if out.MaxProbe <= 0 {
		out.MaxProbe = 16
	}
	if out.MaxDecode <= 0 {
		out.MaxDecode = 4
	}
	if out.MaxEncode <= 0 {
		out.MaxEncode = 2
	}
	if out.LargeFileThresholdBytes <= 0 {
		out.LargeFileThresholdBytes = defaultLargeFileThreshold
	}
	if out.MaxLargeFile == nil {
		v := 2
		out.MaxLargeFile = &v
	}
	return out
}

// LegacyFromMaxConcurrent maps the deprecated single-pool MaxConcurrent setting.
func LegacyFromMaxConcurrent(max int) Config {
	if max < 1 {
		max = 4
	}
	probe := max * 4
	if probe < 16 {
		probe = 16
	}
	return Config{
		MaxProbe:  probe,
		MaxDecode: max,
		MaxEncode: max,
	}
}

// Limiter enforces tiered subprocess concurrency.
type Limiter struct {
	cfg     Config
	probe   chan struct{}
	decode  chan struct{}
	encode  chan struct{}
	global  chan struct{}
	large   chan struct{}
	largeOn bool
}

// Lease holds acquired concurrency slots until Release is called.
type Lease struct {
	limiter *Limiter
	class   SlotClass
	large   bool
	global  bool
	tier    bool
}

// Release frees all slots held by the lease.
func (l *Lease) Release() {
	if l == nil || l.limiter == nil || !l.tier {
		return
	}
	releaseChan(l.limiter.tierChan(l.class))
	if l.large {
		releaseChan(l.limiter.large)
		l.large = false
	}
	if l.global {
		releaseChan(l.limiter.global)
		l.global = false
	}
	l.tier = false
}

// NewLimiter builds a limiter from cfg (defaults applied).
func NewLimiter(cfg Config) *Limiter {
	cfg = cfg.WithDefaults()
	l := &Limiter{
		cfg:    cfg,
		probe:  make(chan struct{}, cfg.MaxProbe),
		decode: make(chan struct{}, cfg.MaxDecode),
		encode: make(chan struct{}, cfg.MaxEncode),
	}
	if cfg.GlobalMax > 0 {
		l.global = make(chan struct{}, cfg.GlobalMax)
	}
	if cfg.MaxLargeFile != nil && *cfg.MaxLargeFile > 0 {
		l.large = make(chan struct{}, *cfg.MaxLargeFile)
		l.largeOn = true
	}
	return l
}

// AcquireLease reserves global, optional large-file, and tier slots for inputPath.
func (l *Limiter) AcquireLease(ctx context.Context, class SlotClass, inputPath string) (*Lease, error) {
	lease := &Lease{limiter: l, class: class}
	if l.global != nil {
		if err := acquireChan(ctx, l.global); err != nil {
			return nil, err
		}
		lease.global = true
	}
	if l.largeOn && needsLargeFileSlot(inputPath, l.cfg.LargeFileThresholdBytes) {
		if err := acquireChan(ctx, l.large); err != nil {
			lease.Release()
			return nil, err
		}
		lease.large = true
	}
	if err := acquireChan(ctx, l.tierChan(class)); err != nil {
		lease.Release()
		return nil, err
	}
	lease.tier = true
	return lease, nil
}

// Acquire waits for a slot in class. Use Release with the same class when done.
// Acquire does not apply the large-file tier; use AcquireLease or Run for input-aware limits.
func (l *Limiter) Acquire(ctx context.Context, class SlotClass) error {
	if l.global != nil {
		if err := acquireChan(ctx, l.global); err != nil {
			return err
		}
	}
	if err := acquireChan(ctx, l.tierChan(class)); err != nil {
		if l.global != nil {
			releaseChan(l.global)
		}
		return err
	}
	return nil
}

// Release frees a slot acquired with Acquire.
func (l *Limiter) Release(class SlotClass) {
	releaseChan(l.tierChan(class))
	if l.global != nil {
		releaseChan(l.global)
	}
}

// Run acquires global (if configured), optional large-file, and tier slots for inputPath, then runs fn.
func (l *Limiter) Run(ctx context.Context, class SlotClass, inputPath string, fn func() error) error {
	lease, err := l.AcquireLease(ctx, class, inputPath)
	if err != nil {
		return err
	}
	defer lease.Release()
	return fn()
}

func (l *Limiter) tierChan(class SlotClass) chan struct{} {
	switch class {
	case SlotProbe:
		return l.probe
	case SlotEncode:
		return l.encode
	default:
		return l.decode
	}
}

func acquireChan(ctx context.Context, ch chan struct{}) error {
	select {
	case ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseChan(ch chan struct{}) {
	<-ch
}

func needsLargeFileSlot(path string, threshold int64) bool {
	if path == "" || threshold <= 0 {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > threshold
}

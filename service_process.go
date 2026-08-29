package ffmpeg

import (
	"context"

	"github.com/gtsteffaniak/go-ffmpeg/concurrency"
)

func (s *Service) ensureDetected(ctx context.Context) error {
	s.mu.RLock()
	if s.caps != nil {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	s.detectMu.Lock()
	defer s.detectMu.Unlock()

	s.mu.RLock()
	if s.caps != nil {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	return s.Reload(ctx)
}

func (s *Service) acquireLease(ctx context.Context, class concurrency.SlotClass, inputPath string) (*concurrency.Lease, error) {
	return s.limiter.AcquireLease(ctx, class, inputPath)
}

func (s *Service) acquireSlot(ctx context.Context, class concurrency.SlotClass) error {
	return s.limiter.Acquire(ctx, class)
}

func (s *Service) releaseSlot(class concurrency.SlotClass) {
	s.limiter.Release(class)
}

func (s *Service) runWithClass(ctx context.Context, class concurrency.SlotClass, inputPath string, fn func() error) error {
	return s.limiter.Run(ctx, class, inputPath, fn)
}

func hlsSlotClass(opts HLSSegmentOptions) concurrency.SlotClass {
	if opts.Remux || opts.VideoCopy {
		return concurrency.SlotDecode
	}
	return concurrency.SlotEncode
}

func hlsContinuousSlotClass(opts HLSContinuousOptions) concurrency.SlotClass {
	if opts.Remux || opts.VideoCopy {
		return concurrency.SlotDecode
	}
	return concurrency.SlotEncode
}

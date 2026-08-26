package ffmpeg

import (
	"context"
)

func (s *Service) ensureDetected(ctx context.Context) error {
	s.mu.RLock()
	if s.caps != nil {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()
	return s.Reload(ctx)
}

func (s *Service) acquireSlot(ctx context.Context) error {
	return s.Acquire(ctx)
}

func (s *Service) releaseSlot() {
	s.Release()
}

func (s *Service) runWithSlot(ctx context.Context, fn func() error) error {
	if err := s.acquireSlot(ctx); err != nil {
		return err
	}
	defer s.releaseSlot()
	return fn()
}

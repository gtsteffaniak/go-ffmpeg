package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestShouldCacheDetectErr(t *testing.T) {
	t.Parallel()
	if !shouldCacheDetectErr(nil) {
		t.Error("success must be cached")
	}
	if !shouldCacheDetectErr(errors.New("ffmpeg not found")) {
		t.Error("a host failure must be cached")
	}
	if shouldCacheDetectErr(context.Canceled) {
		t.Error("cancellation must not be cached")
	}
	if shouldCacheDetectErr(fmt.Errorf("detect: %w", context.DeadlineExceeded)) {
		t.Error("wrapped deadline must not be cached")
	}
}

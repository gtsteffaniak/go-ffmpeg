//go:build windows

package exec

import "context"

// RunFFmpegProcessGroup runs ffmpeg on Windows (no process-group kill; uses RunFFmpeg).
func (r *Runner) RunFFmpegProcessGroup(ctx context.Context, args ...string) (Result, error) {
	return r.RunFFmpeg(ctx, args...)
}

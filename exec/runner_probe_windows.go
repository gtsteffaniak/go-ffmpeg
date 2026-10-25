//go:build windows

package exec

import "context"

// RunFFmpegProcessGroup runs ffmpeg on Windows without Unix process groups (uses RunFFmpeg).
func (r *Runner) RunFFmpegProcessGroup(ctx context.Context, args ...string) (Result, error) {
	return r.RunFFmpeg(ctx, args...)
}

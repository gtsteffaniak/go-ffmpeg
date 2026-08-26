package exec

import (
	"context"
	"io"
	"strings"
)

// Executor runs ffmpeg/ffprobe subprocesses. *Runner implements this interface.
type Executor interface {
	RunFFmpeg(ctx context.Context, args ...string) (Result, error)
	RunFFprobe(ctx context.Context, args ...string) (Result, error)
	FFmpegLogLevel() string
	FFmpegStderrWriter(capture *strings.Builder) io.Writer
}

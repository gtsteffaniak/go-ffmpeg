package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Result holds command output.
type Result struct {
	Stdout string
	Stderr string
}

// Runner executes ffmpeg/ffprobe commands.
type Runner struct {
	FFmpegPath    string
	FFprobePath   string
	VerboseFFmpeg bool
}

// FFmpegLogLevel returns the -loglevel value for ffmpeg invocations.
func (r *Runner) FFmpegLogLevel() string {
	if r != nil && r.VerboseFFmpeg {
		return "info"
	}
	return "warning"
}

// FFmpegStderrWriter returns stderr sink for ffmpeg: os.Stderr when verbose, else a buffer.
func (r *Runner) FFmpegStderrWriter(capture *strings.Builder) io.Writer {
	if r != nil && r.VerboseFFmpeg {
		return os.Stderr
	}
	if capture != nil {
		return capture
	}
	return &strings.Builder{}
}

// RunFFmpeg executes ffmpeg with args.
func (r *Runner) RunFFmpeg(ctx context.Context, args ...string) (Result, error) {
	return r.run(ctx, r.FFmpegPath, args...)
}

// RunFFprobe executes ffprobe with args.
func (r *Runner) RunFFprobe(ctx context.Context, args ...string) (Result, error) {
	return r.run(ctx, r.FFprobePath, args...)
}

func (r *Runner) run(ctx context.Context, bin string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = err.Error()
		}
		return res, fmt.Errorf("%w: %s", err, msg)
	}
	return res, nil
}

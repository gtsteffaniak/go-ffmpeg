package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// RunFFmpegProcessGroup runs ffmpeg in its own process group so timeouts can SIGKILL the whole tree.
func (r *Runner) RunFFmpegProcessGroup(ctx context.Context, args ...string) (Result, error) {
	return r.runProcessGroup(ctx, r.FFmpegPath, args...)
}

func (r *Runner) runProcessGroup(ctx context.Context, bin string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	var stderrSink io.Writer = &stderr
	if r != nil && r.VerboseFFmpeg {
		stderrSink = io.MultiWriter(os.Stderr, &stderr)
	}
	cmd.Stderr = stderrSink

	if err := cmd.Start(); err != nil {
		return Result{}, err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
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
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		err := <-waitCh
		res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
		if err != nil && ctx.Err() == nil {
			return res, err
		}
		return res, ctx.Err()
	}
}

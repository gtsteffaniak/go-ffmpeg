package exec

import (
	"context"
	"os/exec"
)

// CommandContext is a thin wrapper around exec.CommandContext for streaming ops.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

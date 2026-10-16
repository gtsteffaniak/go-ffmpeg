package platform

import (
	"context"
	"os/exec"
	"time"
)

// platformCommandTimeout bounds lspci, lsmod, nvidia-smi, and similar helper invocations.
var platformCommandTimeout = 5 * time.Second

func combinedOutputTimeout(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), platformCommandTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func runSilentTimeout(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), platformCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

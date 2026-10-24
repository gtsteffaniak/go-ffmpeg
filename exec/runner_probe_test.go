package exec

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunFFmpegProcessGroupTimeout(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "slow-ffmpeg.sh")
	content := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	r := &Runner{FFmpegPath: script}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := r.RunFFmpegProcessGroup(ctx, "-hide_banner")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if ctx.Err() == nil {
		t.Fatal("expected context deadline")
	}
}

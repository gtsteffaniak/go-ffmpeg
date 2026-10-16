package ops_test

import (
	"context"
	"strings"
	"testing"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

func TestProtocolWhitelistRejectsConcatURL(t *testing.T) {
	path, err := ffexec.ResolveBinary("", "ffmpeg")
	if err != nil {
		t.Skip(err)
	}
	runner := &ffexec.Runner{FFmpegPath: path}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := runner.AppendProtocolWhitelistArgs([]string{"-hide_banner", "-nostats"})
	args = append(args, "-i", "concat:/etc/passwd", "-f", "null", "-")
	_, err = runner.RunFFmpeg(ctx, args...)
	if err == nil {
		t.Fatal("expected concat: input to fail")
	}
	if !strings.Contains(err.Error(), "whitelist") && !strings.Contains(err.Error(), "Protocol") {
		t.Fatalf("expected whitelist rejection, got: %v", err)
	}
}

func TestProtocolWhitelistRejectsSubfileURL(t *testing.T) {
	path, err := ffexec.ResolveBinary("", "ffmpeg")
	if err != nil {
		t.Skip(err)
	}
	runner := &ffexec.Runner{FFmpegPath: path}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := runner.AppendProtocolWhitelistArgs([]string{"-hide_banner", "-nostats"})
	args = append(args, "-i", "subfile:,start,0,end,10,:file:///etc/passwd", "-f", "null", "-")
	_, err = runner.RunFFmpeg(ctx, args...)
	if err == nil {
		t.Fatal("expected subfile: input to fail")
	}
	if !strings.Contains(err.Error(), "whitelist") && !strings.Contains(err.Error(), "Protocol") {
		t.Fatalf("expected whitelist rejection, got: %v", err)
	}
}

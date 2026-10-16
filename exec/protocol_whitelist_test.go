package exec_test

import (
	"testing"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

func TestAppendProtocolWhitelistDefault(t *testing.T) {
	args := (&ffexec.Runner{}).AppendProtocolWhitelistArgs(nil)
	if len(args) != 2 || args[0] != "-protocol_whitelist" || args[1] != ffexec.DefaultProtocolWhitelist {
		t.Fatalf("got %v", args)
	}
}

func TestAppendProtocolWhitelistDisable(t *testing.T) {
	r := &ffexec.Runner{ProtocolWhitelist: ffexec.DisableProtocolWhitelist}
	args := r.AppendProtocolWhitelistArgs([]string{"-hide_banner"})
	if len(args) != 1 {
		t.Fatalf("expected whitelist disabled, got %v", args)
	}
}

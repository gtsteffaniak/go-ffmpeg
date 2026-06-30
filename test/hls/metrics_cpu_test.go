package main

import (
	"os"
	"testing"
)

func TestProcessTreePIDsIncludesSelf(t *testing.T) {
	pid := os.Getpid()
	tree := processTreePIDs(pid)
	if len(tree) == 0 {
		t.Fatal("expected at least root pid")
	}
	found := false
	for _, p := range tree {
		if p == pid {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tree %v missing root %d", tree, pid)
	}
}

func TestReadProcessTreeCPUPercentSelf(t *testing.T) {
	// Spin briefly so ps reports non-zero for this process.
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_ = 1 + 1
			}
		}
	}()
	defer close(done)

	pct := readProcessTreeCPUPercent(os.Getpid())
	if pct < 0 {
		t.Fatalf("negative cpu percent: %v", pct)
	}
}

func TestIntegrateCPUTimeSec(t *testing.T) {
	samples := []resourceSample{
		{CPUPercent: 200},
		{CPUPercent: 300},
	}
	got := integrateCPUTimeSec(samples, 0.2)
	want := (2.0 + 3.0) * 0.2
	if got != want {
		t.Fatalf("integrateCPUTimeSec = %v, want %v", got, want)
	}
}

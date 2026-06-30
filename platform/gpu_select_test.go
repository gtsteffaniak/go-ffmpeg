package platform_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestResolveGPUDisabledWhenEmpty(t *testing.T) {
	t.Parallel()
	choice, err := platform.ResolveGPU("")
	if err != nil {
		t.Fatal(err)
	}
	if choice.Enabled {
		t.Fatal("expected hardware disabled for empty gpu")
	}
}

func TestNameMatches(t *testing.T) {
	t.Parallel()
	if !platform.NameMatchesForTest("NVIDIA GeForce RTX 4090", "geforce") {
		t.Fatal("expected substring match")
	}
}

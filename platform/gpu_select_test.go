package platform_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestResolveGPUAutoWhenEmpty(t *testing.T) {
	t.Parallel()
	choice, err := platform.ResolveGPURequestDefaultEnv(platform.GPURequest{Kind: platform.SelectionAuto, Label: "auto"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if choice.Enabled || !choice.AutoSelect || choice.Mode != platform.GPUModeAuto {
		t.Fatalf("expected auto, got %+v", choice)
	}
}

func TestNameMatches(t *testing.T) {
	t.Parallel()
	if !platform.NameMatchesForTest("NVIDIA GeForce RTX 4090", "geforce") {
		t.Fatal("expected substring match")
	}
}

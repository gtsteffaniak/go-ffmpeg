package ffmpeg

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestWithDefaultsLeavesEncoderHierarchyEmpty(t *testing.T) {
	cfg := (&Config{}).withDefaults()
	if len(cfg.EncoderHierarchy) != 0 {
		t.Fatalf("EncoderHierarchy = %v, want nil/empty so Detect uses HierarchyForPlatform", cfg.EncoderHierarchy)
	}
}

func TestWithDefaultsPreservesEncoderHierarchy(t *testing.T) {
	custom := []capabilities.AccelType{capabilities.AccelNVENC, capabilities.AccelQSV}
	cfg := (&Config{EncoderHierarchy: custom}).withDefaults()
	if len(cfg.EncoderHierarchy) != 2 || cfg.EncoderHierarchy[0] != capabilities.AccelNVENC {
		t.Fatalf("EncoderHierarchy = %v, want custom override preserved", cfg.EncoderHierarchy)
	}
}

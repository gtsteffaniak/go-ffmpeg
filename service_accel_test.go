package ffmpeg

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

// An empty GPU means "no GPU scoping", not "software only": the platform default
// hierarchy still applies and hardware encoders are preferred when available.
// Use EncoderHierarchy to prefer software instead.
func TestBuildDetectOptionsEmptyGPUSkipsScoping(t *testing.T) {
	svc := &Service{cfg: Config{GPU: ""}}

	opts := svc.buildDetectOptions()

	if opts.GPU.Enabled {
		t.Fatalf("empty GPU should not scope to a device, got %+v", opts.GPU)
	}
	if opts.RenderDevice != "" {
		t.Fatalf("empty GPU should not set a render device, got %q", opts.RenderDevice)
	}
}

// A software-only hierarchy is forwarded untouched, so callers can disable
// hardware acceleration without skipping detection.
func TestBuildDetectOptionsForwardsEncoderHierarchy(t *testing.T) {
	svc := &Service{cfg: Config{EncoderHierarchy: []capabilities.AccelType{capabilities.AccelNone}}}

	opts := svc.buildDetectOptions()

	if len(opts.EncoderHierarchy) != 1 || opts.EncoderHierarchy[0] != capabilities.AccelNone {
		t.Fatalf("encoder hierarchy = %v, want [none]", opts.EncoderHierarchy)
	}
	if opts.GPU.Enabled {
		t.Fatalf("no GPU spec should mean no scoping, got %+v", opts.GPU)
	}
}

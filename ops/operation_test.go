package ops_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
)

func TestSupportedProbeStream(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Protocols["file"] = true
	caps.Protocols["rtsp"] = true
	ok, reasons := ops.Supported(probeStreamOpTest{}, caps)
	if !ok {
		t.Fatalf("expected supported, reasons=%v", reasons)
	}
}

func TestSupportedTranscodeDecodeOnly(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.BuildProfile = capabilities.BuildDecodeOnly
	caps.Protocols["file"] = true
	for _, op := range ops.All() {
		if op.Name() == "Transcode" {
			ok, reasons := ops.Supported(op, caps)
			if ok {
				t.Fatal("transcode should be unsupported on decode-only")
			}
			if len(reasons) == 0 {
				t.Fatal("expected reasons")
			}
		}
	}
}

func TestEvaluateOps(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.BuildProfile = capabilities.BuildFull
	caps.Protocols["file"] = true
	caps.Protocols["http"] = true
	caps.Protocols["https"] = true
	caps.Protocols["rtsp"] = true
	caps.Protocols["hls"] = true
	caps.Protocols["tcp"] = true
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Available: true}
	caps.Encoders["mjpeg"] = capabilities.EncoderCapability{Available: true}
	caps.Filters["scale"] = true
	caps.Filters["segment"] = true
	caps.Filters["concat"] = true
	caps.Filters["fps"] = true
	caps.Filters["tile"] = true
	caps.Filters["transpose"] = true
	ops.EvaluateOps(caps)
	if len(caps.EnabledOps) == 0 {
		t.Fatal("expected enabled ops")
	}
}

// Deprecated operations stay callable but must not be advertised in the
// capability report's enabled/disabled lists.
func TestEvaluateOpsExcludesDeprecatedFromAdvertised(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.BuildProfile = capabilities.BuildFull
	for _, p := range []string{"file", "tcp", "http", "https", "rtsp", "hls"} {
		caps.Protocols[p] = true
	}
	caps.Encoders["mjpeg"] = capabilities.EncoderCapability{Available: true}
	for _, f := range []string{"scale", "segment", "concat", "fps", "tile", "transpose"} {
		caps.Filters[f] = true
	}
	ops.EvaluateOps(caps)

	for _, name := range caps.EnabledOps {
		if name == "Screenshot" || name == "VideoPreview" {
			t.Fatalf("deprecated op %s advertised in EnabledOps: %v", name, caps.EnabledOps)
		}
	}
	if _, ok := caps.DisabledOps["Screenshot"]; ok {
		t.Fatalf("deprecated op Screenshot advertised in DisabledOps")
	}

	// Backward compatibility: calling the deprecated op still resolves.
	deprecated := 0
	for _, op := range ops.All() {
		if op.Name() == "Screenshot" || op.Name() == "VideoPreview" {
			deprecated++
			if !ops.IsDeprecated(op) {
				t.Fatalf("%s should be marked deprecated", op.Name())
			}
			if ok, _ := ops.Supported(op, caps); !ok {
				t.Fatalf("%s should remain supported for existing callers", op.Name())
			}
		}
	}
	if deprecated != 2 {
		t.Fatalf("found %d deprecated ops, want 2", deprecated)
	}
	if len(caps.DeprecatedOps) != 2 {
		t.Fatalf("DeprecatedOps = %v, want 2 entries", caps.DeprecatedOps)
	}
}

type probeStreamOpTest struct{}

func (probeStreamOpTest) Name() string { return "ProbeStream" }
func (probeStreamOpTest) Requirements() ops.RequirementSet {
	return ops.RequirementSet{Protocols: []string{"file", "rtsp"}}
}

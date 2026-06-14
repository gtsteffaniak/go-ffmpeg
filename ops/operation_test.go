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

type probeStreamOpTest struct{}

func (probeStreamOpTest) Name() string { return "ProbeStream" }
func (probeStreamOpTest) Requirements() ops.RequirementSet {
	return ops.RequirementSet{Protocols: []string{"file", "rtsp"}}
}

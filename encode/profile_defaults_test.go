package encode_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestApplyEncodeDefaults(t *testing.T) {
	t.Parallel()

	base := encode.VideoProfile{Codec: encode.CodecH264}
	qsv := encode.ApplyEncodeDefaults(base, capabilities.AccelQSV)
	if qsv.Accel != capabilities.AccelQSV {
		t.Fatalf("Accel = %q, want qsv", qsv.Accel)
	}

	sw := encode.ApplyEncodeDefaults(base, capabilities.AccelNone)
	if !sw.ForceSoftware {
		t.Fatal("expected ForceSoftware for AccelNone default")
	}

	explicit := encode.ApplyEncodeDefaults(encode.VideoProfile{Codec: encode.CodecH264, Accel: capabilities.AccelNVENC}, capabilities.AccelQSV)
	if explicit.Accel != capabilities.AccelNVENC {
		t.Fatalf("explicit Accel overwritten: %q", explicit.Accel)
	}
}

func TestApplyDecodeDefaults(t *testing.T) {
	t.Parallel()

	base := encode.VideoDecodeProfile{Codec: capabilities.CodecH264}
	qsv := encode.ApplyDecodeDefaults(base, capabilities.AccelQSV)
	if qsv.Accel != capabilities.AccelQSV {
		t.Fatalf("Accel = %q, want qsv", qsv.Accel)
	}

	forced := encode.ApplyDecodeDefaults(encode.VideoDecodeProfile{ForceSoftware: true, Codec: capabilities.CodecH264}, capabilities.AccelQSV)
	if forced.Accel != "" || !forced.ForceSoftware {
		t.Fatalf("ForceSoftware profile modified: %+v", forced)
	}
}

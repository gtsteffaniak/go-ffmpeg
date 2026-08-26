package main

import (
	"context"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestTranscodeHLSSegmentParamsSoftwareDecodeOnMac(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Skip(err)
	}
	file := "test/data/Big_Buck_Bunny_1080_10s_2MB.mp4"
	info, err := svc.ProbeFile(ctx, file)
	if err != nil {
		t.Skip(err)
	}
	params, err := transcodeHLSSegmentParams(ctx, svc, file, info, capabilities.AccelNone)
	if err != nil {
		t.Fatal(err)
	}
	if !params.Profile.ForceSoftware {
		t.Fatal("expected software encode")
	}
	if !params.Decode.ForceSoftware {
		t.Fatalf("software transcode must use CPU decode, got %+v", params.Decode)
	}
}

func TestTranscodeHLSSegmentParamsWMVSoftwareDecode(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Skip(err)
	}
	file := ".fixtures/wmv3_wmapro_wmv.wmv"
	info, err := svc.ProbeFile(ctx, file)
	if err != nil {
		t.Skip("wmv fixture missing")
	}
	params, err := transcodeHLSSegmentParams(ctx, svc, file, info, capabilities.AccelNone)
	if err != nil {
		t.Fatal(err)
	}
	if !params.Decode.ForceSoftware {
		t.Fatalf("wmv transcode decode = %+v, want ForceSoftware", params.Decode)
	}
	if params.Decode.Codec != "" {
		t.Fatalf("wmv decode codec = %q, want empty for ffmpeg auto-select", params.Decode.Codec)
	}
}

func TestTranscodeHLSSegmentParamsWMVSkipsHardware(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Skip(err)
	}
	file := ".fixtures/wmv3_wmapro_wmv.wmv"
	info, err := svc.ProbeFile(ctx, file)
	if err != nil {
		t.Skip("wmv fixture missing")
	}
	_, err = transcodeHLSSegmentParams(ctx, svc, file, info, capabilities.AccelVideoToolbox)
	if err == nil {
		t.Fatal("expected hardware transcode to be rejected for wmv input")
	}
}

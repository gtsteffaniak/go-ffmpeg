//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestTranscodeHLSSegmentParamsWMVSoftwareDecode(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Skip(err)
	}
	file := ".fixtures/wmv3_wmapro_wmv.wmv"
	info, err := svc.ProbeFile(ctx, file)
	if err != nil {
		t.Fatalf("probe %s: %v", file, err)
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
		t.Fatalf("probe %s: %v", file, err)
	}
	_, err = transcodeHLSSegmentParams(ctx, svc, file, info, capabilities.AccelVideoToolbox)
	if err == nil {
		t.Fatal("expected hardware transcode to be rejected for wmv input")
	}
}

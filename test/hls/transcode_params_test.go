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
		t.Fatal(err)
	}
	file := defaultSampleVideo()
	info, err := svc.ProbeFile(ctx, file)
	if err != nil {
		t.Fatalf("probe %s: %v", file, err)
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

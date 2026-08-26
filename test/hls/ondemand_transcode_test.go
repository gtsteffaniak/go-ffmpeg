//go:build integration

package main

import (
	"context"
	"testing"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestOnDemandSoftwareTranscodeSegment(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Skip(err)
	}

	file := ".fixtures/h264_aac_mp4.mp4"
	info, err := svc.ProbeFile(ctx, file)
	if err != nil {
		t.Fatalf("probe %s: %v", file, err)
	}

	params, err := transcodeHLSSegmentParams(ctx, svc, file, info, capabilities.AccelNone)
	if err != nil {
		t.Fatal(err)
	}
	if !params.Decode.ForceSoftware {
		t.Fatalf("decode = %+v, want ForceSoftware", params.Decode)
	}
	plan := svc.DescribeHLSSegmentPlan(params)
	if plan == "" {
		t.Fatal("empty plan")
	}
	t.Log("plan:", plan)

	opts := goffmpeg.BuildHLSSegmentOptions(file, 0, params, nil, nil, false, nil, goffmpeg.DefaultHLSSegmentDurationSec)
	init, media, err := svc.HLSInitAndSegment(ctx, opts)
	if err != nil {
		t.Fatalf("HLSInitAndSegment: %v", err)
	}
	if len(init) == 0 || len(media) == 0 {
		t.Fatalf("init=%d media=%d", len(init), len(media))
	}
}

func TestOnDemandSoftwareTranscodeWMVSegment(t *testing.T) {
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
	opts := goffmpeg.BuildHLSSegmentOptions(file, 0, params, nil, nil, false, nil, goffmpeg.DefaultHLSSegmentDurationSec)
	init, media, err := svc.HLSInitAndSegment(ctx, opts)
	if err != nil {
		t.Fatalf("HLSInitAndSegment: %v", err)
	}
	if len(init) == 0 || len(media) == 0 {
		t.Fatalf("init=%d media=%d", len(init), len(media))
	}
}

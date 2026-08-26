//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

const h264FixturePath = ".fixtures/h264_aac_mp4.mp4"

func TestParamsForVariantH264SoftwareDecode(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Skip(err)
	}

	info, err := svc.ProbeFile(ctx, h264FixturePath)
	if err != nil {
		t.Fatalf("probe %s: %v", h264FixturePath, err)
	}

	variant := testVariant{Mode: "transcode", Accel: capabilities.AccelNone, Label: "transcode/software"}
	params, err := paramsForVariant(ctx, svc, h264FixturePath, info, variant)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("decode=%+v profile=%+v", params.Decode, params.Profile)
	if !params.Decode.ForceSoftware {
		t.Fatalf("decode ForceSoftware=false: %+v", params.Decode)
	}
	if !params.Profile.ForceSoftware {
		t.Fatalf("profile ForceSoftware=false: %+v", params.Profile)
	}
}

func TestRunBenchmarkH264SoftwareTranscode(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Skip(err)
	}

	variant := testVariant{Mode: "transcode", Accel: capabilities.AccelNone, Label: "transcode/software"}
	br, err := runBenchmark(ctx, svc, h264FixturePath, "h264_aac_mp4", variant, 1, 0.05, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if br.Skipped {
		t.Fatalf("unexpected skip: %s", br.SkipReason)
	}
	if !br.Pass {
		t.Fatalf("benchmark failed: %s", br.EncodeError)
	}
}

func TestRunBenchmarkH264RemuxMultiSegment(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Skip(err)
	}

	variant := testVariant{Mode: "remux", Label: "remux"}
	br, err := runBenchmark(ctx, svc, h264FixturePath, "h264_aac_mp4", variant, 3, 0.05, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if br.Skipped {
		t.Fatalf("unexpected skip: %s", br.SkipReason)
	}
	if !br.Pass {
		t.Fatalf("remux benchmark failed: %v", br.Issues)
	}
}

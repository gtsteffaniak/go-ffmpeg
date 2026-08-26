package main

import (
	"context"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestParamsForVariantH264SoftwareDecode(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Skip(err)
	}

	path := ".fixtures/h264_aac_mp4.mp4"
	info, err := svc.ProbeFile(ctx, path)
	if err != nil {
		t.Skip("fixture missing")
	}

	variant := testVariant{Mode: "transcode", Accel: capabilities.AccelNone, Label: "transcode/software"}
	params, err := paramsForVariant(ctx, svc, path, info, variant)
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

	path := ".fixtures/h264_aac_mp4.mp4"
	variant := testVariant{Mode: "transcode", Accel: capabilities.AccelNone, Label: "transcode/software"}
	br, err := runBenchmark(ctx, svc, path, "h264_aac_mp4", variant, 1, 0.05, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if br.Skipped {
		t.Skip(br.SkipReason)
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

	path := ".fixtures/h264_aac_mp4.mp4"
	variant := testVariant{Mode: "remux", Label: "remux"}
	br, err := runBenchmark(ctx, svc, path, "h264_aac_mp4", variant, 3, 0.05, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if br.Skipped {
		t.Skip(br.SkipReason)
	}
	if !br.Pass {
		t.Fatalf("remux benchmark failed: %v", br.Issues)
	}
}

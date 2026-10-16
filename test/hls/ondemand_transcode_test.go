//go:build integration

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

func TestOnDemandSoftwareTranscodeSegment(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Fatal(err)
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
	trackTimescales := mp4.TrackTimescalesFromInit(init)
	issues := mp4.ValidateSegmentTimelineWithTimescales(media, mp4.SegmentTimeline{
		Index:            0,
		MediaStartSec:    0,
		ExpectedStartSec: 0,
		ExpectedDurSec:   goffmpeg.DefaultHLSSegmentDurationSec,
	}, mp4.DefaultHLSTimeToleranceSec, trackTimescales)
	if len(issues) > 0 {
		t.Fatalf("timeline issues: %+v", issues)
	}
}

func TestOnDemandSoftwareTranscodeWMVSegment(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	file := ".fixtures/wmv2_wmapro_wmv.wmv"
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
	trackTimescales := mp4.TrackTimescalesFromInit(init)
	issues := mp4.ValidateSegmentTimelineWithTimescales(media, mp4.SegmentTimeline{
		Index:            0,
		MediaStartSec:    0,
		ExpectedStartSec: 0,
		ExpectedDurSec:   goffmpeg.DefaultHLSSegmentDurationSec,
	}, mp4.DefaultHLSTimeToleranceSec, trackTimescales)
	if len(issues) > 0 {
		t.Fatalf("timeline issues: %+v", issues)
	}
}

func TestOnDemandHLSSegmentMPEGTS(t *testing.T) {
	ctx := context.Background()
	svc, err := initFFmpeg(ctx, false)
	if err != nil {
		t.Fatal(err)
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
	starts, durations := goffmpeg.BuildHLSSegmentTimeline(info.Duration, nil, goffmpeg.DefaultHLSSegmentDurationSec)
	midIndex := len(starts) / 2
	if midIndex < 1 {
		midIndex = 1
	}

	ffprobeBin, err := resolveFFprobeBin()
	if err != nil {
		t.Fatal(err)
	}

	for _, idx := range []int{0, midIndex} {
		idx := idx
		t.Run(fmt.Sprintf("segment_%d", idx), func(t *testing.T) {
			opts := goffmpeg.BuildHLSSegmentOptions(file, idx, params, starts, durations, false, nil, goffmpeg.DefaultHLSSegmentDurationSec)
			data, err := svc.HLSSegment(ctx, opts)
			if err != nil {
				t.Fatalf("HLSSegment: %v", err)
			}
			if len(data) < 4096 || data[0] != 0x47 {
				t.Fatalf("segment %d: want MPEG-TS sync 0x47, got %d bytes prefix 0x%02x", idx, len(data), data[0])
			}
			path := filepath.Join(t.TempDir(), "seg.ts")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(ffprobeBin, "-v", "error", "-show_entries", "format=format_name", "-of", "default=nw=1:nk=1", path)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("ffprobe: %v\n%s", err, out)
			}
			format := strings.TrimSpace(string(out))
			if !strings.Contains(format, "mpegts") {
				t.Fatalf("format_name = %q, want mpegts", format)
			}
		})
	}
}

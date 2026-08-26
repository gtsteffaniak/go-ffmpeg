package mp4

import "testing"

func TestValidateSegmentTimelinePass(t *testing.T) {
	t.Parallel()
	media := sampleSingleMoofMedia(uint32(4.0 * defaultVideoTimescale))
	issues := ValidateSegmentTimeline(media, SegmentTimeline{
		Index:            1,
		ExpectedStartSec: 4.0,
		ExpectedDurSec:   2.0,
	}, DefaultHLSTimeToleranceSec)
	for _, issue := range issues {
		if issue.Check == "tfdt_start" || issue.Check == "duration_match" {
			t.Fatalf("unexpected issue: %+v", issue)
		}
	}
}

func TestValidateSegmentTimelineTFDTMismatch(t *testing.T) {
	t.Parallel()
	media := sampleSingleMoofMedia(uint32(4.138 * defaultVideoTimescale))
	issues := ValidateSegmentTimeline(media, SegmentTimeline{
		Index:            1,
		ExpectedStartSec: 4.0,
	}, DefaultHLSTimeToleranceSec)
	found := false
	for _, issue := range issues {
		if issue.Check == "tfdt_start" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected tfdt_start mismatch")
	}
}

func TestValidateContinuityGap(t *testing.T) {
	t.Parallel()
	prev := SegmentTimeline{Index: 0, MediaStartSec: 0, ActualDurSec: 4.088}
	next := SegmentTimeline{Index: 1, MediaStartSec: 4.200}
	issues := ValidateContinuity(prev, next, DefaultHLSTimeToleranceSec)
	if len(issues) != 1 || issues[0].Check != "continuity" {
		t.Fatalf("expected continuity gap, got %+v", issues)
	}
}

func TestFragmentMediaStartSec(t *testing.T) {
	t.Parallel()
	media := sampleSingleMoofMedia(uint32(8.276 * defaultVideoTimescale))
	got, err := FragmentMediaStartSec(media)
	if err != nil {
		t.Fatal(err)
	}
	if got < 8.275 || got > 8.277 {
		t.Fatalf("start = %.3f, want ~8.276", got)
	}
}

func TestFragmentMediaStartSecWithHLSFMP4Timescale(t *testing.T) {
	t.Parallel()
	const hlsVideoTS = uint32(12288) // ffmpeg HLS fMP4 video mdhd (not 90000)
	media := sampleSingleMoofMedia(uint32(4.0 * float64(hlsVideoTS)))
	misread, err := FragmentMediaStartSec(media)
	if err != nil {
		t.Fatal(err)
	}
	if misread < 0.54 || misread > 0.55 {
		t.Fatalf("90kHz misread = %.3f, want ~0.546", misread)
	}
	got, err := FragmentMediaStartSecWithTimescales(media, map[uint32]uint32{1: hlsVideoTS})
	if err != nil {
		t.Fatal(err)
	}
	if got < 3.99 || got > 4.01 {
		t.Fatalf("start = %.3f, want 4.000", got)
	}
}

func TestValidateSegmentTimelineWithHLSFMP4Timescale(t *testing.T) {
	t.Parallel()
	const hlsVideoTS = uint32(12288)
	media := sampleSingleMoofMedia(uint32(4.0 * float64(hlsVideoTS)))
	issues := ValidateSegmentTimelineWithTimescales(media, SegmentTimeline{
		Index:            1,
		ExpectedStartSec: 4.0,
	}, DefaultHLSTimeToleranceSec, map[uint32]uint32{1: hlsVideoTS})
	for _, issue := range issues {
		if issue.Check == "tfdt_start" {
			t.Fatalf("unexpected tfdt_start with init timescale: %+v", issue)
		}
	}
}

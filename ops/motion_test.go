package ops

import (
	"reflect"
	"testing"
)

func TestBuildMotionFilterFullFrame(t *testing.T) {
	d := ResolveMotionDefaults(MotionDetectOptions{})
	got, err := BuildMotionFilter(d, MotionCrop{})
	if err != nil {
		t.Fatal(err)
	}
	want := "fps=2.000000,scale=320:-2,format=gray,tblend=all_mode=difference,signalstats,metadata=print:key=lavfi.signalstats.YAVG:file=-"
	if got != want {
		t.Fatalf("filter mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestBuildMotionFilterCrop(t *testing.T) {
	d := ResolveMotionDefaults(MotionDetectOptions{})
	got, err := BuildMotionFilter(d, MotionCrop{X: 0.1, Y: 0.2, W: 0.5, H: 0.25})
	if err != nil {
		t.Fatal(err)
	}
	want := "fps=2.000000,crop=iw*0.500000:ih*0.250000:iw*0.100000:ih*0.200000,scale=320:-2,format=gray,tblend=all_mode=difference,signalstats,metadata=print:key=lavfi.signalstats.YAVG:file=-"
	if got != want {
		t.Fatalf("filter mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestMotionCropNormalization(t *testing.T) {
	c, err := MotionCrop{X: 0.9, Y: 0.9, W: 0.5, H: 0.5}.normalizedCrop()
	if err != nil {
		t.Fatal(err)
	}
	if c.X != 0.5 || c.Y != 0.5 {
		t.Fatalf("origin should shift to keep the span inside the frame: %+v", c)
	}
	if c.X+c.W > 1.0001 || c.Y+c.H > 1.0001 {
		t.Fatalf("crop must stay inside the frame: %+v", c)
	}
	edge, err := MotionCrop{X: 1, Y: 0, W: 0.1, H: 0.5}.normalizedCrop()
	if err != nil {
		t.Fatal(err)
	}
	if edge.X != 0.9 || edge.W != 0.1 {
		t.Fatalf("edge crop should shift origin, not zero width: %+v", edge)
	}
	if !(MotionCrop{}).Normalized() {
		t.Fatal("empty crop must report full frame")
	}
	if (MotionCrop{W: 0.5, H: 0.5}).Normalized() {
		t.Fatal("nonzero crop must not report full frame")
	}
}

func TestParseMotionSamples(t *testing.T) {
	out := `frame:0    pts:1       pts_time:0.5
lavfi.signalstats.YAVG=0
frame:1    pts:2       pts_time:1
lavfi.signalstats.YAVG=12.5
frame:2    pts:3       pts_time:1.5
lavfi.signalstats.YAVG=3.25
`
	got := parseMotionSamples(out)
	want := []MotionSample{{TimeSec: 0.5, Score: 0}, {TimeSec: 1, Score: 12.5}, {TimeSec: 1.5, Score: 3.25}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("samples mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestMergeMotionEvents(t *testing.T) {
	d := ResolveMotionDefaults(MotionDetectOptions{SampleFPS: 2, Threshold: 2, MinDurationSec: 0.5})
	samples := []MotionSample{
		{TimeSec: 0, Score: 0},
		{TimeSec: 0.5, Score: 5},
		{TimeSec: 1.0, Score: 6},
		{TimeSec: 1.5, Score: 4},
		{TimeSec: 2.0, Score: 0.5},
		{TimeSec: 2.5, Score: 9},
		{TimeSec: 3.0, Score: 0},
	}
	events := mergeMotionEvents(samples, d)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d: %+v", len(events), events)
	}
	if events[0].StartSec != 0.5 || events[0].EndSec != 2.0 {
		t.Fatalf("first event window wrong: %+v", events[0])
	}
	if events[0].PeakScore != 6 {
		t.Fatalf("first event peak wrong: %+v", events[0])
	}
	// A single 0.5s sample at 2fps expands to exactly the minimum duration and is kept.
	if events[1].StartSec != 2.5 || events[1].EndSec != 3.0 {
		t.Fatalf("second event window wrong: %+v", events[1])
	}
}

func TestResolveMotionDefaultsMinDuration(t *testing.T) {
	d := ResolveMotionDefaults(MotionDetectOptions{})
	if d.MinDurationSec != 0.5 {
		t.Fatalf("omitted MinDurationSec should default to 0.5, got %v", d.MinDurationSec)
	}
	d = ResolveMotionDefaults(MotionDetectOptions{MinDurationSec: -1})
	if d.MinDurationSec != 0 {
		t.Fatalf("negative MinDurationSec should disable filtering, got %v", d.MinDurationSec)
	}
}

func TestMergeMotionEventsDropsShortBlips(t *testing.T) {
	d := ResolveMotionDefaults(MotionDetectOptions{SampleFPS: 2, Threshold: 2, MinDurationSec: 1.0})
	samples := []MotionSample{
		{TimeSec: 5, Score: 9},
		{TimeSec: 5.5, Score: 0},
	}
	if events := mergeMotionEvents(samples, d); len(events) != 0 {
		t.Fatalf("blip under MinDurationSec should be dropped: %+v", events)
	}
}

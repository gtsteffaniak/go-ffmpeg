package main

import "testing"

func TestComputeSegmentTiming(t *testing.T) {
	segs := []segmentBenchmark{
		{Index: 0, EncodeMs: 800},
		{Index: 1, EncodeMs: 400},
		{Index: 2, EncodeMs: 420},
	}
	timing := computeSegmentTiming(segs, 12)
	if timing.ColdSegMs != 800 {
		t.Fatalf("cold=%d want 800", timing.ColdSegMs)
	}
	if timing.WarmAvgSegMs != 410 {
		t.Fatalf("warm avg=%d want 410", timing.WarmAvgSegMs)
	}
	if timing.WarmSegCount != 2 {
		t.Fatalf("warm count=%d want 2", timing.WarmSegCount)
	}
}

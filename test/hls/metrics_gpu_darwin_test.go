//go:build darwin

package main

import (
	"testing"
)

func TestMaxAppleDeviceUtilPercent(t *testing.T) {
	sample := []byte(`"PerformanceStatistics" = {"Device Utilization %"=14,"Renderer Utilization %"=20}
"PerformanceStatistics" = {"Device Utilization %"=42}`)
	got := maxAppleDeviceUtilPercent(sample)
	if got == nil || *got != 42 {
		t.Fatalf("maxAppleDeviceUtilPercent = %v, want 42", got)
	}
}

func TestReadIORegGPUUtilLive(t *testing.T) {
	v := readIORegGPUUtil()
	if v == nil {
		t.Skip("no Device Utilization % in ioreg output on this machine")
	}
	if *v < 0 || *v > 100 {
		t.Fatalf("unexpected utilization %v", *v)
	}
}

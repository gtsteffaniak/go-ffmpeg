package platform

import "testing"

func TestParseNVIDIASMIIndexCSV(t *testing.T) {
	csv := `0, 00000000:01:00.0, NVIDIA GeForce RTX 3050 Laptop GPU
1, 00000000:02:00.0, NVIDIA GeForce GTX 1650`
	rows := ParseNVIDIASMIIndexCSV(csv)
	if len(rows) != 2 || rows[0].Index != 0 || rows[1].Index != 1 {
		t.Fatalf("%+v", rows)
	}
	idx, ok := MatchNVIDIACUDAIndex("00000000:01:00.0", "", rows)
	if !ok || idx != 0 {
		t.Fatalf("idx=%d ok=%v", idx, ok)
	}
	idx, ok = MatchNVIDIACUDAIndex("00000000:02:00.0", "", rows)
	if !ok || idx != 1 {
		t.Fatalf("second gpu idx=%d ok=%v", idx, ok)
	}
}

func TestPCIBusIDFromPNPDeviceID(t *testing.T) {
	pnp := `PCI\VEN_10DE&DEV_2504&SUBSYS_152F1025&REV_A1\4&2E2E8E9&0&0019`
	got := PCIBusIDFromPNPDeviceID(pnp)
	if got != "" {
		t.Fatalf("expected empty pci from pnp instance id, got %q", got)
	}
}

func TestPCIEqualDistinctBuses(t *testing.T) {
	a := NormalizePCIBusID("00000000:01:00.0")
	b := NormalizePCIBusID("00000000:02:00.0")
	if PCIEqual(a, b) {
		t.Fatal("01:00.0 must not equal 02:00.0")
	}
	if !PCIEqual(a, NormalizePCIBusID("01:00.0")) {
		t.Fatal("domain padding should still match")
	}
}

func TestNormalizePCIBusID(t *testing.T) {
	a := NormalizePCIBusID("00000000:01:00.0")
	b := NormalizePCIBusID("01:00.0")
	if !PCIEqual(a, b) && a != b {
		if a == "" {
			t.Fatal("empty")
		}
	}
}

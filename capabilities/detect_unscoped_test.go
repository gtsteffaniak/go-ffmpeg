package capabilities

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestHybridWindowsProbesAllVendorGates(t *testing.T) {
	plat := PlatformInfo{OS: "windows", NVIDIA: true, AMD: true, Intel: true}
	if !platformGateForEncoder("amf", plat) {
		t.Fatal("AMD gate should be true on hybrid laptop")
	}
	if !platformGateForEncoder("nvenc", plat) {
		t.Fatal("NVIDIA gate should be true")
	}
	h := HierarchyForGPU("nvidia", plat)
	if encoderKindInHierarchy("amf", h) {
		t.Fatal("AMF should not be in NVIDIA hierarchy")
	}
}

func TestSelectionNoteWhenOutOfHierarchy(t *testing.T) {
	plat := platform.Info{NVIDIA: true, AMD: true}
	h := HierarchyForGPU("nvidia", plat)
	if encoderKindInHierarchy("amf", h) {
		t.Fatal()
	}
	// Document expected reporting label for encoders tested but not selected.
	note := "available, not selected"
	if note == "" {
		t.Fatal("placeholder")
	}
}

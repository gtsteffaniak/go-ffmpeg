package encode_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestAppendReadrateArgs(t *testing.T) {
	v5 := capabilities.Version{Major: 5, Minor: 0, Patch: 0}
	v8 := capabilities.Version{Major: 8, Minor: 0, Patch: 0}

	disabled := encode.AppendReadrateArgs(nil, v8, encode.ThrottleConfig{Enabled: false})
	if len(disabled) != 0 {
		t.Fatalf("disabled throttle should not append args: %v", disabled)
	}

	v5Args := encode.AppendReadrateArgs(nil, v5, encode.ThrottleConfig{Enabled: true})
	if len(v5Args) != 2 || v5Args[0] != "-readrate" || v5Args[1] != "1" {
		t.Fatalf("v5 args = %v", v5Args)
	}

	v8Args := encode.AppendReadrateArgs(nil, v8, encode.ThrottleConfig{Enabled: true, Rate: 1.5, Catchup: 3})
	want := []string{"-readrate", "1.5", "-readrate_catchup", "3"}
	if len(v8Args) != len(want) {
		t.Fatalf("v8 args = %v, want %v", v8Args, want)
	}
	for i := range want {
		if v8Args[i] != want[i] {
			t.Fatalf("v8 args = %v, want %v", v8Args, want)
		}
	}
}

func TestAppendReadrateArgsVersionGates(t *testing.T) {
	cfg := encode.ThrottleConfig{
		Enabled:      true,
		Rate:         1,
		Catchup:      2,
		InitialBurst: 30,
	}

	v60 := capabilities.Version{Major: 6, Minor: 0, Patch: 0}
	v60Args := encode.AppendReadrateArgs(nil, v60, cfg)
	if len(v60Args) != 2 {
		t.Fatalf("6.0 should omit burst and catchup: %v", v60Args)
	}

	v61 := capabilities.Version{Major: 6, Minor: 1, Patch: 0}
	v61Args := encode.AppendReadrateArgs(nil, v61, cfg)
	if len(v61Args) != 4 || v61Args[2] != "-readrate_initial_burst" {
		t.Fatalf("6.1 should add initial_burst: %v", v61Args)
	}

	v7 := capabilities.Version{Major: 7, Minor: 0, Patch: 0}
	v7Args := encode.AppendReadrateArgs(nil, v7, cfg)
	for _, arg := range v7Args {
		if arg == "-readrate_catchup" {
			t.Fatalf("7.0 should not add catchup: %v", v7Args)
		}
	}

	v8 := capabilities.Version{Major: 8, Minor: 0, Patch: 0}
	v8Args := encode.AppendReadrateArgs(nil, v8, cfg)
	hasCatchup := false
	for _, arg := range v8Args {
		if arg == "-readrate_catchup" {
			hasCatchup = true
		}
	}
	if !hasCatchup {
		t.Fatalf("8.0 should add catchup: %v", v8Args)
	}
}

func TestFailureClassifier(t *testing.T) {
	c := encode.FailureClassifier{}
	tests := []struct {
		stderr string
		kind   encode.FailureKind
	}{
		{"Error opening input: No such file or directory", encode.FailureInputNotFound},
		{"Invalid data found when processing input", encode.FailureInputInvalid},
		{"Unknown encoder 'libfoo'", encode.FailureEncoder},
		{"Conversion failed!", encode.FailureEncode},
		{"Output file is empty", encode.FailureOutputEmpty},
		{"Device setup failed for encoder", encode.FailureHardware},
	}
	for _, tc := range tests {
		got := c.Classify(tc.stderr)
		if got.Kind != tc.kind {
			t.Errorf("Classify(%q) kind = %q, want %q", tc.stderr, got.Kind, tc.kind)
		}
	}
}

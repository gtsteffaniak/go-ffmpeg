package encode_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestThrottleConfigPresets(t *testing.T) {
	off := encode.ThrottleConfigOff()
	if off.Enabled {
		t.Fatal("ThrottleConfigOff should be disabled")
	}

	live := encode.ThrottleConfigLivePaced(0)
	if !live.Enabled || live.Rate != 1.0 || live.Catchup != 2.0 || live.InitialBurst != encode.DefaultLivePacedBurstSec {
		t.Fatalf("ThrottleConfigLivePaced(0) = %+v", live)
	}
	live90 := encode.ThrottleConfigLivePaced(90)
	if live90.InitialBurst != 90 {
		t.Fatalf("InitialBurst = %v", live90.InitialBurst)
	}

	remux := encode.ThrottleConfigRemuxSegmentDeletion()
	if !remux.Enabled || remux.Rate != 10 || remux.Catchup != 1000 {
		t.Fatalf("ThrottleConfigRemuxSegmentDeletion = %+v", remux)
	}
}

func TestAppendReadrateArgsLivePaced(t *testing.T) {
	v8 := capabilities.Version{Major: 8, Minor: 0, Patch: 0}
	cfg := encode.ThrottleConfigLivePaced(60)
	args := encode.AppendReadrateArgs(nil, v8, cfg)
	want := []string{"-readrate", "1", "-readrate_catchup", "2", "-readrate_initial_burst", "60"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args = %v, want %v", args, want)
		}
	}
}

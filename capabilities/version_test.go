package capabilities_test

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		raw     string
		want    capabilities.Version
		wantErr bool
	}{
		{"8.1.1", capabilities.Version{8, 1, 1}, false},
		{"n8.1.1", capabilities.Version{8, 1, 1}, false},
		{"5.0", capabilities.Version{5, 0, 0}, false},
		{"4.4.2", capabilities.Version{4, 4, 2}, false},
		{"7.0.0", capabilities.Version{7, 0, 0}, false},
		{"", capabilities.Version{}, true},
		{"invalid", capabilities.Version{}, true},
	}
	for _, tc := range tests {
		got, err := capabilities.ParseSemver(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseSemver(%q) expected error", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSemver(%q): %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseSemver(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestCompare(t *testing.T) {
	v5 := capabilities.Version{5, 0, 0}
	v8 := capabilities.Version{8, 0, 0}
	v49 := capabilities.Version{4, 9, 9}
	if capabilities.Compare(v5, v8) >= 0 {
		t.Fatal("5.0.0 should be less than 8.0.0")
	}
	if capabilities.Compare(v49, v5) >= 0 {
		t.Fatal("4.9.9 should be less than 5.0.0")
	}
	if capabilities.Compare(v5, v5) != 0 {
		t.Fatal("5.0.0 should equal 5.0.0")
	}
}

func TestFeatureFlagsFromVersion(t *testing.T) {
	flags49 := capabilities.FeatureFlagsFromVersion(capabilities.Version{4, 9, 9})
	if flags49.NoiseBSFDrop || flags49.Readrate || flags49.ReadrateCatchup || flags49.InputSideBSF {
		t.Fatalf("4.9.9 flags should be false: %+v", flags49)
	}

	flags5 := capabilities.FeatureFlagsFromVersion(capabilities.Version{5, 0, 0})
	if !flags5.NoiseBSFDrop || !flags5.Readrate {
		t.Fatalf("5.0.0 should enable noise BSF and readrate: %+v", flags5)
	}
	if flags5.ReadrateCatchup || flags5.InputSideBSF {
		t.Fatalf("5.0.0 should not enable catchup or input-side BSF: %+v", flags5)
	}

	flags7 := capabilities.FeatureFlagsFromVersion(capabilities.Version{7, 0, 0})
	if !flags7.InputSideBSF {
		t.Fatal("7.0.0 should enable input-side BSF")
	}
	if flags7.ReadrateCatchup {
		t.Fatal("7.0.0 should not enable readrate catchup")
	}

	flags8 := capabilities.FeatureFlagsFromVersion(capabilities.Version{8, 1, 1})
	if !flags8.ReadrateCatchup {
		t.Fatal("8.1.1 should enable readrate catchup")
	}
	if !flags8.NoiseBSFDrop || !flags8.Readrate || !flags8.InputSideBSF {
		t.Fatalf("8.1.1 should enable all flags: %+v", flags8)
	}
}

func TestMinSupportedVersion(t *testing.T) {
	if capabilities.MinSupportedVersion != (capabilities.Version{5, 0, 0}) {
		t.Fatalf("MinSupportedVersion = %v", capabilities.MinSupportedVersion)
	}
}

package ops

import (
	"testing"
)

func TestParsePacketKeyframeCSVFieldsTrimsTrailingComma(t *testing.T) {
	t.Parallel()
	fields := parsePacketKeyframeCSVFields("1.483333,1.400000,K__,")
	if len(fields) != 3 {
		t.Fatalf("fields = %#v, want 3", fields)
	}
	if fields[2] != "K__" {
		t.Fatalf("flags = %q", fields[2])
	}
}

func TestPacketKeyframeTimeFromCSVLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line string
		want float64
		ok   bool
	}{
		{"0.000000,-0.083333,K__", 0, true},
		{"1.483333,1.400000,K__,", 1.483333, true},
		{"N/A,0.000000,K__", 0, true},
		{"N/A,N/A,K__", 0, false},
		{"0.166667,-0.041667,___", 0, false},
	}
	for _, tc := range cases {
		got, ok := packetKeyframeTimeFromCSVLine(tc.line)
		if ok != tc.ok {
			t.Fatalf("line %q: ok = %v, want %v", tc.line, ok, tc.ok)
		}
		if ok && got != tc.want {
			t.Fatalf("line %q: time = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestSortDedupeKeyframeTimes(t *testing.T) {
	t.Parallel()
	got := sortDedupeKeyframeTimes([]float64{2, 1.0005, 1, 3})
	want := []float64{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: %v vs %v", i, got[i], want[i])
		}
	}
}

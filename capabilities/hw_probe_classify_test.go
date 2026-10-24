package capabilities

import (
	"context"
	"testing"
)

func TestClassifyHWProbeFailure(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		kind   string
		want   HWProbeCode
	}{
		{
			name:   "vaapi permission",
			stderr: "Failed to initialise VAAPI: Permission denied opening /dev/dri/renderD128",
			kind:   "vaapi",
			want:   HWProbeDevicePermission,
		},
		{
			name:   "libva missing",
			stderr: "Cannot load libva.so.2",
			kind:   "vaapi",
			want:   HWProbeLibvaMissing,
		},
		{
			name:   "va error -12",
			stderr: "Failed to initialise VAAPI: Unknown libva error -12",
			kind:   "vaapi",
			want:   HWProbeVADriverInitFailed,
		},
		{
			name:   "nvenc libs",
			stderr: "Cannot load libnvidia-encode.so.1",
			kind:   "nvenc",
			want:   HWProbeNvidiaLibsMissing,
		},
		{
			name:   "nvenc unsupported",
			stderr: "OpenEncodeSessionEx failed: unsupported device (2)",
			kind:   "nvenc",
			want:   HWProbeNVENCUnsupported,
		},
		{
			name:   "nvenc sessions",
			stderr: "Too many concurrent sessions",
			kind:   "nvenc",
			want:   HWProbeNVENCSessionLimit,
		},
		{
			name:   "timeout",
			stderr: "probe timed out",
			kind:   "vaapi",
			want:   HWProbeTimeout,
		},
	}
	for _, tc := range cases {
		code, _ := ClassifyHWProbeFailure(tc.stderr, nil, tc.kind)
		if code != tc.want {
			t.Fatalf("%s: code=%s want %s", tc.name, code, tc.want)
		}
	}
	code, _ := ClassifyHWProbeFailure("", context.DeadlineExceeded, "nvenc")
	if code != HWProbeTimeout {
		t.Fatalf("deadline: got %s", code)
	}
}

package capabilities

import (
	"context"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// AV1 on Ampere (and any codec without an NVENC unit) must not be promoted to
// gpu_present_driver_missing, even though an NVIDIA GPU is present on WSL.
func TestClassifyAndRemediateAV1UnsupportedNotPromoted(t *testing.T) {
	code, hint := ClassifyHWProbeFailure("Codec not supported\nNo capable devices found", nil, "nvenc")
	if code != HWProbeNVENCUnsupported {
		t.Fatalf("code=%s want %s", code, HWProbeNVENCUnsupported)
	}
	host := DetectionHost{
		Presence: platform.GPUPresence{NVIDIA: true, WSLDXG: true},
		WSL:      true,
	}
	finalCode, finalHint := ApplyHostRemediation(host, "nvenc", FFmpegBinaryInfo{Kind: FFmpegLinkDynamic}, code, hint)
	if finalCode != HWProbeNVENCUnsupported {
		t.Fatalf("final code=%s want %s (must not promote)", finalCode, HWProbeNVENCUnsupported)
	}
	if finalHint != hint {
		t.Fatalf("hint changed from %q to %q", hint, finalHint)
	}
}

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
		{
			// AV1 on Ampere: GPU has no NVENC AV1 unit. Must classify as a
			// capability limit, not a missing device/driver.
			name:   "nvenc av1 codec not supported",
			stderr: "Codec not supported\nNo capable devices found",
			kind:   "nvenc",
			want:   HWProbeNVENCUnsupported,
		},
		{
			// WSL Vulkan: extension absent, reported as ENOSYS / -38.
			name:   "vulkan function not implemented",
			stderr: "[h264_vulkan] Device does not support the VK_KHR_video_encode_queue extension!\nTask finished with error code: -38 (Function not implemented)",
			kind:   "vulkan",
			want:   HWProbeVulkanNoVideoEncode,
		},
		{
			name:   "vulkan error code -38",
			stderr: "Task finished with error code: -38 (Function not implemented)",
			kind:   "vulkan",
			want:   HWProbeVulkanNoVideoEncode,
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

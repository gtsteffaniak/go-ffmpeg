package platform

import "testing"

func TestVAAPIVendorDriver(t *testing.T) {
	dri := func(names ...string) []string {
		out := make([]string, len(names))
		for i, n := range names {
			out[i] = "/usr/lib/x86_64-linux-gnu/dri/" + n
		}
		return out
	}
	cases := []struct {
		name        string
		all         []string
		intel, amd  bool
		wantDriver  string
		wantBlocked bool
	}{
		{"intel iHD", dri("iHD_drv_video.so", "radeonsi_drv_video.so"), true, false, "/usr/lib/x86_64-linux-gnu/dri/iHD_drv_video.so", false},
		{"intel legacy i965", dri("i965_drv_video.so"), true, false, "/usr/lib/x86_64-linux-gnu/dri/i965_drv_video.so", false},
		{"amd radeonsi", dri("radeonsi_drv_video.so"), false, true, "/usr/lib/x86_64-linux-gnu/dri/radeonsi_drv_video.so", false},
		// Hybrid: Intel iGPU without its media driver must not mask a usable
		// AMD driver on the same box.
		{"hybrid intel driverless + amd", dri("radeonsi_drv_video.so"), true, true, "/usr/lib/x86_64-linux-gnu/dri/radeonsi_drv_video.so", false},
		// Both vendors present, matching drivers found for each.
		{"hybrid both drivers", dri("iHD_drv_video.so", "r600_drv_video.so"), true, true, "/usr/lib/x86_64-linux-gnu/dri/iHD_drv_video.so", false},
		{"intel no driver", dri("radeonsi_drv_video.so"), true, false, "", true},
		{"amd no driver", dri("iHD_drv_video.so"), false, true, "", true},
		{"no vendor gpus", dri("nvidia_drv_video.so"), false, false, "", false},
		{"empty driver list", nil, true, true, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			driver, blocked := vaapiVendorDriver(tc.all, tc.intel, tc.amd)
			if driver != tc.wantDriver || blocked != tc.wantBlocked {
				t.Fatalf("vaapiVendorDriver = (%q, %v), want (%q, %v)", driver, blocked, tc.wantDriver, tc.wantBlocked)
			}
		})
	}
}

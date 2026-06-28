package capabilities_test

import (
	"os"
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestParseVersionOutput(t *testing.T) {
	data, err := os.ReadFile("../testdata/ffmpeg-8.1.1-version.txt")
	if err != nil {
		t.Fatal(err)
	}
	version, cfg := capabilities.ParseVersionOutput(string(data))
	if version != "8.1.1" {
		t.Fatalf("version = %q", version)
	}
	if len(cfg.LibFlags) == 0 {
		t.Fatal("expected lib flags")
	}
	foundX264 := false
	for _, f := range cfg.LibFlags {
		if f == "--enable-libx264" {
			foundX264 = true
		}
	}
	if !foundX264 {
		t.Fatal("expected libx264 flag")
	}
}

func TestParseListOutput(t *testing.T) {
	data, err := os.ReadFile("../testdata/ffmpeg-8.1.1-encoders.txt")
	if err != nil {
		t.Fatal(err)
	}
	names := capabilities.ParseListOutput(string(data))
	want := map[string]bool{"libx264": true, "libsvtav1": true, "mjpeg": true}
	for name := range want {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing encoder %s in %v", name, names)
		}
	}
}

func TestParseHWAccelsOutput(t *testing.T) {
	out := "Hardware acceleration methods:\nvdpau\ncuda\nvaapi\nqsv\n"
	names := capabilities.ParseHWAccelsOutput(out)
	want := map[string]bool{"vdpau": true, "cuda": true, "vaapi": true, "qsv": true}
	for name := range want {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing hwaccel %s in %v", name, names)
		}
	}
}

func TestCodecEncoderMap(t *testing.T) {
	if got := capabilities.CodecEncoderMap(capabilities.CodecH264, capabilities.AccelNVENC); got != "h264_nvenc" {
		t.Fatalf("got %q", got)
	}
}

func TestCodecDecoderMap(t *testing.T) {
	if got := capabilities.CodecDecoderMap(capabilities.CodecH264, capabilities.AccelNVENC); got != "h264_cuvid" {
		t.Fatalf("got %q", got)
	}
	if got := capabilities.CodecHWAccelDecodeKey(capabilities.CodecVP9, capabilities.AccelVAAPI); got != "hwaccel:vaapi:vp9" {
		t.Fatalf("got %q", got)
	}
}

func TestCodecEncoderMapD3D12(t *testing.T) {
	if got := capabilities.CodecEncoderMap(capabilities.CodecH264, capabilities.AccelD3D12); got != "h264_vaapi" {
		t.Fatalf("d3d12 maps to vaapi, got %q", got)
	}
	if got := capabilities.CodecEncoderMap(capabilities.CodecH264, capabilities.AccelVAAPI); got != "h264_vaapi" {
		t.Fatalf("vaapi maps to h264_vaapi, got %q", got)
	}
}

func TestHierarchyForPlatform(t *testing.T) {
	linux := capabilities.HierarchyForPlatform(capabilities.PlatformInfo{OS: "linux", Intel: true, VAAPI: true})
	if len(linux) != 4 || linux[0] != capabilities.AccelNVENC || linux[1] != capabilities.AccelVAAPI ||
		linux[2] != capabilities.AccelQSV || linux[3] != capabilities.AccelAMF {
		t.Fatalf("Linux hierarchy = %v, want NVENC→VAAPI→QSV→AMF", linux)
	}
	windows := capabilities.HierarchyForPlatform(capabilities.PlatformInfo{OS: "windows", Intel: true, QSV: true})
	if len(windows) != 3 || windows[0] != capabilities.AccelNVENC || windows[1] != capabilities.AccelQSV || windows[2] != capabilities.AccelAMF {
		t.Fatalf("Windows hierarchy = %v, want NVENC→QSV→AMF", windows)
	}
	for _, accel := range windows {
		if accel == capabilities.AccelVAAPI {
			t.Fatal("Windows hierarchy must not include VAAPI")
		}
	}
	wsl := capabilities.HierarchyForPlatform(capabilities.PlatformInfo{OS: "linux", WSL: true, D3D12: true})
	if len(wsl) != 4 || wsl[0] != capabilities.AccelNVENC || wsl[1] != capabilities.AccelD3D12 ||
		wsl[2] != capabilities.AccelVAAPI || wsl[3] != capabilities.AccelQSV {
		t.Fatalf("WSL hierarchy = %v, want NVENC→D3D12→VAAPI→QSV", wsl)
	}
	darwin := capabilities.HierarchyForPlatform(capabilities.PlatformInfo{OS: "darwin"})
	if len(darwin) != 1 || darwin[0] != capabilities.AccelVideoToolbox {
		t.Fatalf("macOS hierarchy = %v, want VideoToolbox only", darwin)
	}
}

func TestBuildCodecMatrix(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Compiled: true, Available: true, Kind: "software"}
	caps.Encoders["h264_nvenc"] = capabilities.EncoderCapability{Name: "h264_nvenc", Compiled: true, Available: true, Kind: "nvenc"}
	caps.Decoders["h264"] = capabilities.DecoderCapability{Name: "h264", Compiled: true, Available: true, Kind: "software"}
	caps.Decoders["h264_cuvid"] = capabilities.DecoderCapability{Name: "h264_cuvid", Compiled: true, Available: true, Kind: "nvenc"}
	caps.Platform.NVIDIA = true
	capabilities.BuildCodecMatrix(caps, capabilities.DefaultHierarchy())
	support := caps.CodecMatrix[capabilities.CodecH264]
	if support.Preferred.Encoder != "h264_nvenc" {
		t.Fatalf("preferred encode = %q", support.Preferred.Encoder)
	}
	if support.DecodePreferred.Decoder != "h264_cuvid" {
		t.Fatalf("preferred decode = %q", support.DecodePreferred.Decoder)
	}
}

func TestInferBuildProfile(t *testing.T) {
	_, cfg := capabilities.ParseVersionOutput("--enable-libx264 --enable-libsvtav1")
	encoders := map[string]capabilities.EncoderCapability{
		"libx264":   {Compiled: true},
		"libsvtav1": {Compiled: true},
	}
	if got := capabilities.InferBuildProfile(cfg, encoders); got != capabilities.BuildFull {
		t.Fatalf("profile = %q", got)
	}
}

func TestReportGroupedByCodec(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.FFmpegVersion = "8.1.1"
	caps.FFmpegPath = "/usr/bin/ffmpeg"
	caps.BuildProfile = capabilities.BuildFull
	caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Compiled: true, Available: true, Kind: "software"}
	caps.Encoders["h264_nvenc"] = capabilities.EncoderCapability{Name: "h264_nvenc", Compiled: true, Available: false, Kind: "nvenc", TestError: "[platform] no NVIDIA GPU/driver detected"}
	caps.Encoders["h264_qsv"] = capabilities.EncoderCapability{Name: "h264_qsv", Compiled: true, Available: false, Kind: "qsv", TestError: "[runtime] Intel Quick Sync session failed (MFX -9)\nUpdate: sudo apt install intel-media-va-driver-non-free libvpl2"}
	caps.Decoders["h264"] = capabilities.DecoderCapability{Name: "h264", Compiled: true, Available: true, Kind: "software"}
	caps.Decoders["h264_qsv"] = capabilities.DecoderCapability{Name: "h264_qsv", Compiled: true, Available: false, Kind: "qsv"}

	report := caps.ReportString()
	if !strings.Contains(report, "H.264 / AVC") {
		t.Fatal("expected H.264 section")
	}
	if !strings.Contains(report, "encode:") || !strings.Contains(report, "decode:") {
		t.Fatal("expected encode/decode labels in report")
	}
	if strings.Contains(report, "H.264 — Intel Quick Sync") {
		t.Fatal("expected grouped backend label without redundant codec prefix")
	}
	if !strings.Contains(report, "Intel Quick Sync") {
		t.Fatal("expected Intel Quick Sync backend label")
	}
}

func TestReportString(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.FFmpegVersion = "8.1.1"
	caps.FFmpegPath = "/usr/bin/ffmpeg"
	caps.BuildProfile = capabilities.BuildFull
	report := caps.ReportString()
	if report == "" {
		t.Fatal("empty report")
	}
}

package capabilities

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// Real stderr captured from Intel Raptor Lake / Tiger Lake and RTX 3050 hosts.
const (
	stderrAV1VAAPI = `[av1_vaapi @ 0x557d2f9eeb80] No usable encoding entrypoint found for profile VAProfileAV1Profile0 (32).
[vost#0:0/av1_vaapi @ 0x557d2f9ee580] [enc:av1_vaapi @ 0x557d2f9eeb00] Error while opening encoder - maybe incorrect parameters such as bit_rate, rate, width or height.
[vost#0:0/av1_vaapi @ 0x557d2f9ee580] Task finished with error code: -22 (Invalid argument)
Conversion failed!`

	stderrAV1QSV = `[av1_qsv @ 0x560fda132200] Using the constant quantization parameter (CQP) by default.
[av1_qsv @ 0x560fda132200] Selected ratecontrol mode is unsupported
[av1_qsv @ 0x560fda132200] Current frame rate is unsupported
[av1_qsv @ 0x560fda132200] Current resolution is unsupported
[av1_qsv @ 0x560fda132200] some encoding parameters are not supported by the QSV runtime. Please double check the input parameters.
Conversion failed!`
)

func TestClassifyAV1VAAPIIsHardwareLimit(t *testing.T) {
	code, _ := ClassifyHWProbeFailure(stderrAV1VAAPI, nil, "vaapi")
	if code != HWProbeCodecUnsupported {
		t.Fatalf("code=%s want %s", code, HWProbeCodecUnsupported)
	}
}

// Parameter-only QSV rejections are not proof of a codec limit on their own.
func TestClassifyQSVParameterMismatchIsNeutral(t *testing.T) {
	code, _ := ClassifyHWProbeFailure(stderrAV1QSV, nil, "qsv")
	if code != HWProbeUnknown {
		t.Fatalf("code=%s want %s", code, HWProbeUnknown)
	}
	code, _ = ClassifyHWProbeFailure("Current codec type is unsupported", nil, "qsv")
	if code != HWProbeCodecUnsupported {
		t.Fatalf("code=%s want %s", code, HWProbeCodecUnsupported)
	}
}

func TestQSVSessionFailureStaysDriverRelated(t *testing.T) {
	code, _ := ClassifyHWProbeFailure("Error creating a MFX session: -9", nil, "qsv")
	if code == HWProbeCodecUnsupported {
		t.Fatal("MFX session failure is a runtime problem, not a codec limit")
	}
}

func TestCodecUnsupportedNeverPromotedToDriverMissing(t *testing.T) {
	host := DetectionHost{Presence: platform.GPUPresence{Intel: true}}
	code, hint := ApplyHostRemediation(host, "vaapi", FFmpegBinaryInfo{}, HWProbeCodecUnsupported, HintForProbeCode(HWProbeCodecUnsupported))
	if code != HWProbeCodecUnsupported {
		t.Fatalf("code=%s, capability limits must not be promoted", code)
	}
	if strings.Contains(hint, "pacman") || strings.Contains(hint, "install") {
		t.Fatalf("hint must not suggest installing packages: %q", hint)
	}
}

// A backend with a working encoder has a working driver stack, so a leftover
// unknown/driver-style failure for another codec is a hardware limit.
func TestDemoteSiblingProbeFailures(t *testing.T) {
	caps := NewCapabilities()
	caps.Encoders["h264_vaapi"] = EncoderCapability{Name: "h264_vaapi", Kind: "vaapi", Tested: true, Available: true}
	caps.Encoders["av1_vaapi"] = EncoderCapability{
		Name: "av1_vaapi", Kind: "vaapi", Tested: true,
		ProbeCode: HWProbeGPUPresentDriverMissing, ProbeOriginal: HWProbeUnknown,
		ProbeHint: "GPU detected (Intel) but VAAPI encode is unavailable on this host; install ...",
	}
	// A genuine driver-init failure is never demoted, even with a working sibling.
	caps.Encoders["hevc_vaapi"] = EncoderCapability{
		Name: "hevc_vaapi", Kind: "vaapi", Tested: true,
		ProbeCode: HWProbeGPUPresentDriverMissing, ProbeOriginal: HWProbeVADriverInitFailed,
	}
	// No working sibling on qsv: stays a driver problem.
	caps.Encoders["h264_qsv"] = EncoderCapability{Name: "h264_qsv", Kind: "qsv", Tested: true, ProbeCode: HWProbeGPUPresentDriverMissing}

	demoteSiblingProbeFailures(caps)

	av1 := caps.Encoders["av1_vaapi"]
	if av1.ProbeCode != HWProbeCodecUnsupported {
		t.Fatalf("av1_vaapi code=%s want %s", av1.ProbeCode, HWProbeCodecUnsupported)
	}
	if strings.Contains(av1.ProbeHint, "install") || strings.Contains(av1.TestError, "install") {
		t.Fatalf("demoted probe must not keep install advice: hint=%q err=%q", av1.ProbeHint, av1.TestError)
	}
	if got := caps.Encoders["hevc_vaapi"].ProbeCode; got != HWProbeGPUPresentDriverMissing {
		t.Fatalf("driver-init failure must be preserved, got %s", got)
	}
	if got := caps.Encoders["h264_qsv"].ProbeCode; got != HWProbeGPUPresentDriverMissing {
		t.Fatalf("h264_qsv must be left alone, got %s", got)
	}
}

func TestStartupInfoLineForCodecLimits(t *testing.T) {
	caps := NewCapabilities()
	caps.EncoderProbes = []EncoderProbeResult{
		{Encoder: "av1_vaapi", Accel: AccelVAAPI, Code: HWProbeCodecUnsupported},
		{Encoder: "av1_qsv", Accel: AccelQSV, Code: HWProbeCodecUnsupported},
		{Encoder: "h264_vaapi", Accel: AccelVAAPI, Available: true},
	}
	if warns := caps.StartupHardwareWarnLines(); len(warns) != 0 {
		t.Fatalf("capability limits must not warn: %v", warns)
	}
	info := caps.StartupHardwareInfoLines()
	if len(info) != 1 || !strings.Contains(info[0], "av1_qsv, av1_vaapi") || strings.Contains(info[0], "install") {
		t.Fatalf("info=%v", info)
	}
}

func TestCompactProbeDetail(t *testing.T) {
	got := compactProbeDetail(stderrAV1VAAPI, 240)
	if strings.Contains(got, "\n") || strings.Contains(got, "0x") {
		t.Fatalf("detail must be single-line without addresses: %q", got)
	}
	if !strings.Contains(got, "No usable encoding entrypoint found for profile VAProfileAV1Profile0") {
		t.Fatalf("detail lost the root cause: %q", got)
	}
	if strings.Contains(got, "Task finished") || strings.Contains(got, "Conversion failed") {
		t.Fatalf("noise lines should be dropped: %q", got)
	}
	long := compactProbeDetail(strings.Repeat("error word ", 100), 50)
	if len(long) > 60 || !strings.HasSuffix(long, "…") {
		t.Fatalf("long detail not cut at boundary: %q", long)
	}
}

func TestCompactProbeDetailDropsDuplicateSummary(t *testing.T) {
	raw := "[hardware] AV1 encode not supported by this GPU's VAAPI driver\n" + stderrAV1VAAPI
	got := compactProbeDetail(raw, 240)
	if strings.Contains(got, "[hardware]") {
		t.Fatalf("summary line duplicates the probe hint: %q", got)
	}
	if only := compactProbeDetail("[hardware] VP9 encode not supported", 240); !strings.Contains(only, "[hardware]") {
		t.Fatalf("summary must be kept when nothing else exists: %q", only)
	}
}

func TestFormatProbeErrorCutsAtWordBoundary(t *testing.T) {
	detail := strings.Repeat("Current resolution is unsupported; ", 10)
	msg := formatProbeError(HWProbeCodecUnsupported, "hint", detail)
	if strings.Contains(msg, "...") {
		t.Fatalf("must not cut mid-word with '...': %q", msg)
	}
	if strings.Contains(msg, "unsuppo…") || strings.Contains(msg, "Curr…") {
		t.Fatalf("cut mid-word: %q", msg)
	}
}

func summaryCaps() *Capabilities {
	caps := NewCapabilities()
	caps.EncoderHierarchy = []AccelType{AccelVAAPI, AccelQSV}
	for _, name := range []string{"h264_vaapi", "hevc_vaapi", "h264_qsv", "hevc_qsv"} {
		kind := "vaapi"
		if strings.HasSuffix(name, "qsv") {
			kind = "qsv"
		}
		caps.Encoders[name] = EncoderCapability{Name: name, Kind: kind, Tested: true, Available: true}
	}
	for _, c := range []VideoCodec{CodecH264, CodecHEVC, CodecAV1, CodecVP9} {
		caps.CodecMatrix[c] = CodecSupport{DecodePreferred: DecoderSelection{Decoder: "hwaccel:vaapi:" + string(c), Accel: AccelVAAPI}}
	}
	caps.EncoderProbes = []EncoderProbeResult{
		{Encoder: "h264_vaapi", Accel: AccelVAAPI, Available: true},
		{Encoder: "av1_vaapi", Accel: AccelVAAPI, Code: HWProbeCodecUnsupported},
		{Encoder: "av1_qsv", Accel: AccelQSV, Code: HWProbeCodecUnsupported},
		{Encoder: "vp9_vaapi", Accel: AccelVAAPI, Code: HWProbeCodecUnsupported},
	}
	return caps
}

func TestHardwareSupportSummaryInfoForCapabilityLimits(t *testing.T) {
	line, warn := summaryCaps().HardwareSupportSummary()
	if warn {
		t.Fatalf("GPU capability limits must not warn: %q", line)
	}
	want := "hardware encode: VAAPI [h264 hevc], QSV [h264 hevc] | decode: h264 hevc av1 vp9 | no hw encode: av1 vp9 (GPU limit)"
	if line != want {
		t.Fatalf("line=%q\nwant=%q", line, want)
	}
}

func TestHardwareSupportSummaryWarnsWhenDriverFixable(t *testing.T) {
	caps := NewCapabilities()
	caps.EncoderProbes = []EncoderProbeResult{{
		Encoder: "h264_nvenc", Accel: AccelNVENC, Code: HWProbeGPUPresentDriverMissing,
		Hint: "GPU detected (NVIDIA) but NVENC encode is unavailable on this host; install the proprietary NVIDIA driver including libnvidia-encode.so.1",
	}, {
		Encoder: "hevc_nvenc", Accel: AccelNVENC, Code: HWProbeGPUPresentDriverMissing,
		Hint: "GPU detected (NVIDIA) but NVENC encode is unavailable on this host; install the proprietary NVIDIA driver including libnvidia-encode.so.1",
	}}
	line, warn := caps.HardwareSupportSummary()
	if !warn {
		t.Fatalf("driver-fixable gap must warn: %q", line)
	}
	want := "missing driver: NVENC | fix: install the proprietary NVIDIA driver including libnvidia-encode.so.1"
	if line != want {
		t.Fatalf("line=%q\nwant=%q", line, want)
	}
}

func TestHardwareSupportSummarySoftwareOnly(t *testing.T) {
	line, warn := NewCapabilities().HardwareSupportSummary()
	if warn || line != "hardware encode: none (software only)" {
		t.Fatalf("line=%q warn=%v", line, warn)
	}
}

func TestCompactProbeDetailFallbackStripsPrefix(t *testing.T) {
	got := compactProbeDetail("[h264_nvenc @ 0xabc123] Device busy\nConversion failed!", 240)
	if strings.Contains(got, "0x") || got != "Device busy" {
		t.Fatalf("fallback must strip the log prefix, got %q", got)
	}
}

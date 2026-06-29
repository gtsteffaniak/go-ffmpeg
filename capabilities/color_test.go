package capabilities

import (
	"strings"
	"testing"
)

func TestVisibleLenIgnoresANSI(t *testing.T) {
	plain := "VideoToolbox"
	styled := "\033[2mSoftware — x264\033[0m"
	if visibleLen(plain) != len(plain) {
		t.Fatalf("plain visibleLen = %d, want %d", visibleLen(plain), len(plain))
	}
	if visibleLen(styled) != len("Software — x264") {
		t.Fatalf("styled visibleLen = %d, want %d", visibleLen(styled), len("Software — x264"))
	}
}

func TestPadVisibleAlignsColumns(t *testing.T) {
	a := padVisible("\033[2mSoftware — x264\033[0m", 26)
	b := padVisible("VideoToolbox", 26)
	if visibleLen(a) != 26 || visibleLen(b) != 26 {
		t.Fatalf("visible widths = %d and %d, want 26", visibleLen(a), visibleLen(b))
	}
}

func TestReportColStyledAlignsWithColor(t *testing.T) {
	plain := "NVIDIA NVENC"
	styled := "\033[32mNVIDIA NVENC\033[0m"
	col := reportColStyled(plain, styled, 26)
	// Column ends with unstyled spaces after the reset sequence.
	if !strings.HasSuffix(col, strings.Repeat(" ", 14+reportColGap)) {
		t.Fatalf("expected 14 pad spaces + gap after styled text, got %q", col)
	}
}

func TestReportEncoderRowColumnAlignment(t *testing.T) {
	for _, colorOn := range []bool{false, true} {
		color := colorOn
		caps := NewCapabilities()
		caps.FFmpegVersion = "8.1.1"
		caps.FFmpegPath = "/usr/bin/ffmpeg"
		caps.Encoders["libx264"] = EncoderCapability{Name: "libx264", Compiled: true, Available: true, Kind: "software"}
		caps.Encoders["h264_videotoolbox"] = EncoderCapability{Name: "h264_videotoolbox", Compiled: true, Available: true, Kind: "videotoolbox"}
		caps.Encoders["h264_nvenc"] = EncoderCapability{Name: "h264_nvenc", Compiled: false, Available: false, Kind: "nvenc"}
		caps.Decoders["h264"] = DecoderCapability{Name: "h264", Available: true}
		caps.Decoders["hwaccel:videotoolbox:h264"] = DecoderCapability{Name: "hwaccel:videotoolbox:h264", Available: true}

		report := caps.ReportStringWithOptions(ReportOptions{Color: &color})
		var rows []string
		for _, line := range strings.Split(report, "\n") {
			if strings.Contains(line, "compiled=") && strings.Contains(line, "encode:") {
				rows = append(rows, line)
			}
		}
		if len(rows) < 2 {
			t.Fatalf("color=%v: expected encoder rows, got %d", colorOn, len(rows))
		}
		compiledAt := columnStart(rows, "compiled=")
		encodeAt := columnStart(rows, "encode:")
		decodeAt := columnStart(rows, "decode:")
		for i, row := range rows {
			if got := columnStart([]string{row}, "compiled="); got != compiledAt {
				t.Fatalf("color=%v row %d compiled= at %d, want %d: %q", colorOn, i, got, compiledAt, row)
			}
			if got := columnStart([]string{row}, "encode:"); got != encodeAt {
				t.Fatalf("color=%v row %d encode: at %d, want %d: %q", colorOn, i, got, encodeAt, row)
			}
			if strings.Contains(row, "decode:") {
				if decodeAt < 0 {
					t.Fatalf("color=%v row %d has decode but no baseline column", colorOn, i)
				}
				if got := columnStart([]string{row}, "decode:"); got != decodeAt {
					t.Fatalf("color=%v row %d decode: at %d, want %d: %q", colorOn, i, got, decodeAt, row)
				}
			}
		}
	}
}

func columnStart(rows []string, marker string) int {
	for _, row := range rows {
		if idx := strings.Index(stripANSI(row), marker); idx >= 0 {
			return idx
		}
	}
	return -1
}

func stripANSI(s string) string {
	return ansiEscapePattern.ReplaceAllString(s, "")
}

package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

func TestResolveFrameSeekSec(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		at   FramePosition
		dur  float64
		want float64
	}{
		{"first frame", FramePosition{}, 120, 0},
		{"seconds", AtSec(12.5), 100, 12.5},
		{"negative seconds error validated separately", AtSec(0), 100, 0},
		{"seconds clamped near end", AtSec(500), 100, 100 - 0.05},
		{"percent", AtPercent(25), 200, 50},
		{"percent clamped above 100", AtPercent(150), 80, 80},
		{"percent without duration", AtPercent(50), 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveFrameSeekSec(tt.at, tt.dur)
			if got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestValidateFramePosition(t *testing.T) {
	t.Parallel()
	if err := ValidateFramePosition(AtSec(-1)); err == nil {
		t.Fatal("expected error for negative seconds")
	}
	if err := ValidateFramePosition(AtPercent(-0.1)); err == nil {
		t.Fatal("expected error for negative percent")
	}
}

func TestFrameNeedsDurationProbe(t *testing.T) {
	t.Parallel()
	if !FrameNeedsDurationProbe(AtPercent(10), 0) {
		t.Fatal("percent without duration should probe")
	}
	if FrameNeedsDurationProbe(AtSec(5), 0) {
		t.Fatal("seconds should not require duration probe")
	}
	if FrameNeedsDurationProbe(AtPercent(10), 99) {
		t.Fatal("known duration should skip probe")
	}
}

func TestPreviewNeedsDurationProbe(t *testing.T) {
	t.Parallel()
	if !PreviewNeedsDurationProbe(PreviewOptions{}) {
		t.Fatal("default preview should probe")
	}
	if PreviewNeedsDurationProbe(PreviewOptions{SeekSec: 3}) {
		t.Fatal("explicit seek sec should skip probe")
	}
	if PreviewNeedsDurationProbe(PreviewOptions{DurationSec: 10}) {
		t.Fatal("known duration should skip probe")
	}
}

func TestFrameCaptureArgsSeekPlacement(t *testing.T) {
	t.Parallel()
	runner := &ffexec.Runner{}
	opts := FrameOptions{Input: InputSource{URL: "in.mp4"}}
	enc := frameEncoder{vcodec: "mjpeg", muxer: "image2pipe"}

	fast := BuildFrameCaptureArgs(runner, opts.Input, nil, frameCaptureArgs(opts, enc, 5))
	joined := strings.Join(fast, " ")
	if !strings.Contains(joined, "-ss 5.000") {
		t.Fatalf("fast seek: %s", joined)
	}
	ssIdx := indexOf(fast, "-ss")
	iIdx := indexOf(fast, "-i")
	if ssIdx < 0 || iIdx < 0 || ssIdx > iIdx {
		t.Fatalf("expected -ss before -i: %v", fast)
	}
	if !sliceContains(fast, "-noaccurate_seek") {
		t.Fatalf("expected fast accuracy: %v", fast)
	}

	opts.Accuracy = CaptureExact
	exact := BuildFrameCaptureArgs(runner, opts.Input, nil, frameCaptureArgs(opts, enc, 5))
	if !sliceContains(exact, "-accurate_seek") {
		t.Fatalf("expected exact accuracy: %v", exact)
	}
}

func TestValidateFrameOptionsFormatGate(t *testing.T) {
	t.Parallel()
	caps := &capabilities.Capabilities{Encoders: map[string]capabilities.EncoderCapability{
		"mjpeg": {Available: true},
	}}
	if err := ValidateFrameOptions(caps, FrameOptions{Input: InputSource{URL: "x"}}); err != nil {
		t.Fatal(err)
	}
	caps.Encoders["png"] = capabilities.EncoderCapability{Available: false}
	err := ValidateFrameOptions(caps, FrameOptions{Input: InputSource{URL: "x"}, Format: FramePNG})
	if err == nil {
		t.Fatal("expected png gate error")
	}
}

func TestCaptureFrameFileAtomicCleanup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.jpg")
	runner := &ffexec.Runner{FFmpegPath: "/nonexistent/ffmpeg"}
	opts := FrameOptions{Input: InputSource{URL: "in.mp4"}, Timeout: time.Second}
	err := CaptureFrameFile(context.Background(), runner, nil, path, opts, 0)
	if err == nil {
		t.Fatal("expected failure")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".goffmpeg-capture-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestPreviewOptionsToFrameLegacyQualityDefault(t *testing.T) {
	t.Parallel()
	fo := PreviewOptionsToFrame(PreviewOptions{}, 0)
	if fo.Quality != LegacyPreviewDefaultQuality {
		t.Fatalf("quality=%d want legacy default %d", fo.Quality, LegacyPreviewDefaultQuality)
	}
	if encode.MJPEGQualityQScale(fo.Quality) != "10" {
		t.Fatalf("legacy preview should map to -q:v 10, got %s", encode.MJPEGQualityQScale(fo.Quality))
	}
}

func TestResolvePreviewSeekSecParityWithCapture(t *testing.T) {
	t.Parallel()
	dur := 200.0
	preview := PreviewOptions{SeekPercent: 25}
	seek := ResolvePreviewSeekSec(dur, preview)
	capSeek := ResolveFrameSeekSec(AtPercent(25), dur)
	if seek != capSeek {
		t.Fatalf("preview %v capture %v", seek, capSeek)
	}
}

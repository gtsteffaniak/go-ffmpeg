package capabilities_test

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

func requireFFmpegRunner(t *testing.T) *ffexec.Runner {
	t.Helper()
	ffmpegBin := os.Getenv("GOFFMPEG_FFMPEG_PATH")
	ffprobeBin := os.Getenv("GOFFMPEG_FFPROBE_PATH")
	if ffmpegBin == "" {
		var err error
		ffmpegBin, err = exec.LookPath("ffmpeg")
		if err != nil {
			t.Fatalf("ffmpeg is required for unit tests: install ffmpeg or set GOFFMPEG_FFMPEG_PATH (%v)", err)
		}
	}
	if ffprobeBin == "" {
		var err error
		ffprobeBin, err = exec.LookPath("ffprobe")
		if err != nil {
			t.Fatalf("ffprobe is required for unit tests: install ffprobe or set GOFFMPEG_FFPROBE_PATH (%v)", err)
		}
	}
	return &ffexec.Runner{FFmpegPath: ffmpegBin, FFprobePath: ffprobeBin}
}

func TestDetectRequiresFFmpeg(t *testing.T) {
	runner := requireFFmpegRunner(t)
	caps, err := capabilities.Detect(context.Background(), runner, capabilities.DetectOptions{})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if caps.FFmpegVersion == "" {
		t.Fatal("missing ffmpeg version")
	}
	t.Logf("platform: wsl=%v gpu_partitioning=%v qsv=%v vaapi=%v nvidia=%v",
		caps.Platform.WSL, caps.Platform.WSLGPUPartitioning, caps.Platform.QSV, caps.Platform.VAAPI, caps.Platform.NVIDIA)

	for _, codec := range []capabilities.VideoCodec{
		capabilities.CodecH264,
		capabilities.CodecAV1,
		capabilities.CodecVP9,
		capabilities.CodecHEVC,
	} {
		support, ok := caps.CodecMatrix[codec]
		if !ok {
			continue
		}
		t.Logf("%s preferred encode: %s (%s)", codec, support.Preferred.Encoder, support.Preferred.Accel)
		for accel, enc := range support.Hardware {
			avail := caps.Encoders[enc].Available
			t.Logf("  hw %s -> %s available=%v", accel, enc, avail)
		}
	}
}

func TestHardwareEncoderSmokeMatchesDetection(t *testing.T) {
	runner := requireFFmpegRunner(t)
	ctx := context.Background()
	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	for _, known := range capabilities.KnownEncoders {
		if !known.HW {
			continue
		}
		enc := caps.Encoders[known.Name]
		if !enc.Compiled {
			continue
		}
		if !enc.Available {
			t.Logf("skip unavailable hw encoder %s: %s", known.Name, enc.TestError)
			continue
		}
		ok, msg := capabilities.SmokeTestHardwareEncoder(ctx, runner, known.Name, caps.Platform)
		if !ok {
			t.Errorf("encoder %s marked available but smoke failed: %s", known.Name, msg)
		}
	}
}

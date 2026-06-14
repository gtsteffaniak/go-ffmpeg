package encode_test

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
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

func TestVideoProfileEncodesAllAvailable(t *testing.T) {
	runner := requireFFmpegRunner(t)
	ctx := context.Background()

	caps, err := capabilities.Detect(ctx, runner, capabilities.DetectOptions{})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	resolver := encode.NewResolver(caps)
	bitrate := encode.BitrateConfig{Target: "1M", Max: "1M", BufSize: "2M", Min: "500k"}
	qualities := []encode.QualityPreset{"", encode.PresetUltrafast, encode.PresetMedium}

	codecs := []encode.VideoCodec{
		encode.CodecH264,
		encode.CodecAV1,
		encode.CodecVP9,
		encode.CodecHEVC,
	}

	for _, codec := range codecs {
		support, ok := caps.CodecMatrix[codec]
		if !ok {
			continue
		}

		for _, q := range qualities {
			for _, sw := range support.Software {
				profile := encode.VideoProfile{
					Codec:   codec,
					Encoder: sw,
					Quality: q,
					Bitrate: bitrate,
					Accel:   capabilities.AccelNone,
				}
				runProfileSmoke(t, ctx, runner, resolver, caps, profile, "software", sw)
			}
		}

		for accel, encName := range support.Hardware {
			encCap := caps.Encoders[encName]
			if !encCap.Available {
				t.Logf("skip %s %s: %s", codec, encName, encCap.TestError)
				continue
			}
			for _, q := range qualities {
				profile := encode.VideoProfile{
					Codec:   codec,
					Accel:   accel,
					Quality: q,
					Bitrate: bitrate,
				}
				runProfileSmoke(t, ctx, runner, resolver, caps, profile, encCap.Kind, encName)
			}
		}
	}
}

func runProfileSmoke(
	t *testing.T,
	ctx context.Context,
	runner *ffexec.Runner,
	resolver *encode.Resolver,
	caps *capabilities.Capabilities,
	profile encode.VideoProfile,
	kind string,
	label string,
) {
	t.Helper()
	qLabel := profile.Quality
	if qLabel == "" {
		qLabel = "default"
	}
	name := string(profile.Codec) + "/" + label + "/quality=" + string(qLabel)

	encArgs, err := resolver.VideoEncoderArgs(profile)
	if err != nil {
		t.Errorf("%s: VideoEncoderArgs: %v", name, err)
		return
	}

	ok, msg := encode.SmokeTestEncoderArgs(ctx, runner, kind, encArgs, caps.Platform)
	if !ok {
		t.Errorf("%s: smoke encode failed: %s\nargs: %v", name, msg, encArgs)
		return
	}
	t.Logf("ok %s", name)
}

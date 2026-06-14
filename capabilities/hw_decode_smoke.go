package capabilities

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

const hwDecodeTestTimeout = 12 * time.Second

type codecFixtures struct {
	paths map[VideoCodec]string
}

func newCodecFixtures() *codecFixtures {
	return &codecFixtures{paths: make(map[VideoCodec]string)}
}

func (f *codecFixtures) cleanup() {
	for _, path := range f.paths {
		_ = os.Remove(path)
	}
}

func (f *codecFixtures) path(ctx context.Context, runner *ffexec.Runner, codec VideoCodec) (string, error) {
	if path, ok := f.paths[codec]; ok {
		return path, nil
	}
	path, err := generateCodecFixture(ctx, runner, codec)
	if err != nil {
		return "", err
	}
	f.paths[codec] = path
	return path, nil
}

func generateCodecFixture(ctx context.Context, runner *ffexec.Runner, codec VideoCodec) (string, error) {
	ext, encodeArgs := codecFixtureEncodeArgs(codec)
	if ext == "" {
		return "", fmt.Errorf("unsupported codec fixture %s", codec)
	}
	dir := os.TempDir()
	file, err := os.CreateTemp(dir, "go-ffmpeg-decode-*"+ext)
	if err != nil {
		return "", err
	}
	path := file.Name()
	_ = file.Close()

	args := []string{
		"-hide_banner", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=0.1:size=320x240:rate=30",
		"-pix_fmt", "yuv420p",
	}
	args = append(args, encodeArgs...)
	args = append(args, path)

	testCtx, cancel := context.WithTimeout(ctx, hwDecodeTestTimeout)
	defer cancel()

	if _, err := runner.RunFFmpeg(testCtx, args...); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func codecFixtureEncodeArgs(codec VideoCodec) (ext string, encodeArgs []string) {
	switch codec {
	case CodecH264:
		return ".h264", []string{"-c:v", "libx264", "-t", "0.1", "-f", "h264"}
	case CodecHEVC:
		return ".hevc", []string{"-c:v", "libx265", "-t", "0.1", "-f", "hevc"}
	case CodecVP9:
		return ".ivf", []string{"-c:v", "libvpx-vp9", "-t", "0.1", "-f", "ivf"}
	case CodecAV1:
		return ".mp4", []string{"-c:v", "libsvtav1", "-t", "0.1", "-f", "mp4"}
	default:
		return "", nil
	}
}

// SmokeTestHardwareDecoder runs a decode smoke test for a hardware decoder path.
func SmokeTestHardwareDecoder(ctx context.Context, runner *ffexec.Runner, decoderName string, codec VideoCodec, hwAccel string, swCodec string, plat platform.Info, fixtures *codecFixtures) (bool, string) {
	bitstream, err := fixtures.path(ctx, runner, codec)
	if err != nil {
		return false, "[runtime] could not generate test bitstream for decode smoke test"
	}

	testCtx, cancel := context.WithTimeout(ctx, hwDecodeTestTimeout)
	defer cancel()

	args := hwDecodeSmokeArgs(decoderName, bitstream, hwAccel, swCodec, plat)
	if args == nil {
		return false, "unsupported hardware decoder for smoke test"
	}

	res, err := runner.RunFFmpeg(testCtx, args...)
	output := res.Stdout + res.Stderr
	if err != nil {
		kind := decoderKindForName(decoderName)
		primary, hints := hwTroubleshoot(decoderName, kind, output, plat)
		if primary == "" {
			primary = summarizeHWError(output)
		}
		if !strings.HasPrefix(primary, "[") {
			primary = "[runtime] " + primary
		}
		return false, formatTestMessages(primary, hints)
	}
	for _, pattern := range hwFailurePatterns {
		if strings.Contains(output, pattern) {
			kind := decoderKindForName(decoderName)
			primary, hints := hwTroubleshoot(decoderName, kind, output, plat)
			if primary == "" {
				primary = summarizeHWError(output)
			}
			return false, formatTestMessages(primary, hints)
		}
	}
	return true, ""
}

func hwDecodeSmokeArgs(decoderName, bitstream, hwAccel, swCodec string, plat platform.Info) []string {
	renderDev := platform.RenderDevice(plat.Details)
	kind := decoderKindForName(decoderName)

	switch kind {
	case "nvenc":
		return []string{
			"-hide_banner",
			"-c:v", decoderName,
			"-i", bitstream,
			"-f", "null", "-",
		}
	case "qsv":
		name := decoderName
		if strings.HasPrefix(name, "hwaccel:") {
			return nil
		}
		return []string{
			"-hide_banner",
			"-init_hw_device", "qsv=hw",
			"-c:v", name,
			"-i", bitstream,
			"-f", "null", "-",
		}
	case "vaapi":
		if hwAccel == "" {
			hwAccel = "vaapi"
		}
		if swCodec == "" && strings.HasPrefix(decoderName, "hwaccel:vaapi:") {
			swCodec = strings.TrimPrefix(decoderName, "hwaccel:vaapi:")
		}
		if swCodec == "" {
			return nil
		}
		return []string{
			"-hide_banner",
			"-init_hw_device", "vaapi=va:" + renderDev,
			"-hwaccel", hwAccel,
			"-hwaccel_device", "va",
			"-c:v", swCodec,
			"-i", bitstream,
			"-f", "null", "-",
		}
	default:
		return []string{
			"-hide_banner",
			"-c:v", decoderName,
			"-i", bitstream,
			"-f", "null", "-",
		}
	}
}

func decoderActuallyCompiled(ctx context.Context, runner *ffexec.Runner, name string, listed bool, hwAccel string, swCodec string, caps *Capabilities) bool {
	if strings.HasPrefix(name, "hwaccel:") {
		if hwAccel == "" {
			hwAccel = "vaapi"
		}
		if swCodec == "" && strings.HasPrefix(name, "hwaccel:vaapi:") {
			swCodec = strings.TrimPrefix(name, "hwaccel:vaapi:")
		}
		if !caps.HWAccels[hwAccel].Compiled {
			return false
		}
		res, err := runner.RunFFmpeg(ctx, "-hide_banner", "-h", "decoder="+swCodec)
		if err != nil {
			return false
		}
		return ParseDecoderHelp(res.Stdout+res.Stderr, swCodec)
	}

	res, err := runner.RunFFmpeg(ctx, "-hide_banner", "-h", "decoder="+name)
	if err != nil {
		return false
	}
	body := res.Stdout + res.Stderr
	if !ParseDecoderHelp(body, name) {
		return false
	}
	lower := strings.ToLower(body)
	if strings.Contains(lower, "unknown decoder") {
		return false
	}
	return listed || strings.Contains(body, "Decoder")
}

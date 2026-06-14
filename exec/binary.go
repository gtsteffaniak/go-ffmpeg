package exec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ResolveBinary resolves an ffmpeg or ffprobe executable path.
func ResolveBinary(providedPath, execName string) (string, error) {
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(strings.ToLower(execName), ".exe") {
			execName += ".exe"
		}
	}

	if providedPath != "" {
		info, err := os.Stat(providedPath)
		if err == nil {
			if info.IsDir() {
				return filepath.Join(providedPath, execName), validateBinary(filepath.Join(providedPath, execName))
			}
			base := filepath.Base(providedPath)
			if strings.EqualFold(base, execName) || strings.EqualFold(base, strings.TrimSuffix(execName, ".exe")) {
				return providedPath, validateBinary(providedPath)
			}
			return filepath.Join(filepath.Dir(providedPath), execName), validateBinary(filepath.Join(filepath.Dir(providedPath), execName))
		}
		return filepath.Join(providedPath, execName), validateBinary(filepath.Join(providedPath, execName))
	}

	path, err := exec.LookPath(strings.TrimSuffix(execName, ".exe"))
	if err != nil {
		return "", err
	}
	return path, validateBinary(path)
}

func validateBinary(path string) error {
	cmd := exec.Command(path, "-version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("invalid binary at %s: %w", path, err)
	}
	return nil
}

// ResolvePair resolves ffmpeg and ffprobe paths.
func ResolvePair(ffmpegPath, ffprobePath string) (ffmpeg, ffprobe string, err error) {
	ffmpeg, err = ResolveBinary(ffmpegPath, "ffmpeg")
	if err != nil {
		return "", "", fmt.Errorf("ffmpeg: %w", err)
	}
	if ffprobePath != "" {
		ffprobe, err = ResolveBinary(ffprobePath, "ffprobe")
	} else {
		ffprobe = siblingBinary(ffmpeg, "ffprobe")
		err = validateBinary(ffprobe)
	}
	if err != nil {
		return "", "", fmt.Errorf("ffprobe: %w", err)
	}
	return ffmpeg, ffprobe, nil
}

func siblingBinary(ffmpegPath, name string) string {
	dir := filepath.Dir(ffmpegPath)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}

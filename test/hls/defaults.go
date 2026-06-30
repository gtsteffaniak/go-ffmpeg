package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const defaultSampleFilename = "Big_Buck_Bunny_1080_10s_2MB.mp4"

// defaultSampleVideo returns the bundled test sample under test/data/.
// Override with HLS_TEST_FILE.
func defaultSampleVideo() string {
	if _, file, _, ok := runtime.Caller(0); ok {
		candidate := filepath.Join(filepath.Dir(file), "..", "data", defaultSampleFilename)
		if _, err := os.Stat(candidate); err == nil {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
			return candidate
		}
	}
	for _, rel := range []string{
		filepath.Join("test", "data", defaultSampleFilename),
		filepath.Join("..", "data", defaultSampleFilename),
	} {
		if _, err := os.Stat(rel); err == nil {
			if abs, err := filepath.Abs(rel); err == nil {
				return abs
			}
			return rel
		}
	}
	return filepath.Join("..", "data", defaultSampleFilename)
}

func resolveFFmpegBin() (string, error) {
	if p := os.Getenv("GOFFMPEG_FFMPEG_PATH"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("GOFFMPEG_FFMPEG_PATH: %w", err)
		}
		return p, nil
	}
	return exec.LookPath("ffmpeg")
}

func resolveFFprobeBin() (string, error) {
	if p := os.Getenv("GOFFMPEG_FFPROBE_PATH"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("GOFFMPEG_FFPROBE_PATH: %w", err)
		}
		return p, nil
	}
	if p := os.Getenv("GOFFMPEG_FFMPEG_PATH"); p != "" {
		candidate := filepath.Join(filepath.Dir(p), "ffprobe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return exec.LookPath("ffprobe")
}

package main

import (
	"os"
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

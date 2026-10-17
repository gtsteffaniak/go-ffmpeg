package ops

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

// HLSVODSegmentPath returns the absolute path to segment index under a VOD job directory.
func HLSVODSegmentPath(outDir string, index int) string {
	return filepath.Join(outDir, fmtHLSVODSegmentPath(index))
}

func fmtHLSVODSegmentPath(index int) string {
	return fmt.Sprintf(HLSVODSegmentNamePattern, index)
}

// HLSVODInitPath returns the absolute path to init.m4s under a VOD job directory.
func HLSVODInitPath(outDir string) string {
	return filepath.Join(outDir, HLSVODInitFilename)
}

// HLSVODSegmentReady reports whether path exists and contains a complete fMP4 media fragment.
func HLSVODSegmentReady(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if len(data) < minHLSSegmentMediaBytes {
		return false, nil
	}
	return mp4.IsHLSSegmentFragment(data), nil
}

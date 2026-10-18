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

const hlsVODSegmentTmpSuffix = ".tmp"

// HLSVODSegmentReady reports whether path exists and contains a publishable fMP4 media fragment.
// A sibling path.tmp means ffmpeg is still writing (hls_flags temp_file); on older ffmpeg builds
// where temp_file does not apply to fMP4, also use HLSVODSegmentReadyAfter or wait for the
// next segment index / job completion before serving.
func HLSVODSegmentReady(path string) (bool, error) {
	if _, err := os.Stat(path + hlsVODSegmentTmpSuffix); err == nil {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return HLSVODSegmentBytesReady(data), nil
}

// HLSVODSegmentBytesReady validates in-memory segment bytes for serving.
func HLSVODSegmentBytesReady(data []byte) bool {
	if len(data) < minHLSSegmentMediaBytes {
		return false
	}
	return mp4.PublishableHLSSegmentFragment(data)
}

// HLSVODSegmentReadyAfter reports readiness for index when the encode head has advanced.
// If nextIndexPath exists and is ready, segment index is treated as complete even when
// temp_file atomic rename is unavailable (older ffmpeg).
func HLSVODSegmentReadyAfter(outDir string, index int) (bool, error) {
	path := HLSVODSegmentPath(outDir, index)
	ready, err := HLSVODSegmentReady(path)
	if err != nil || ready {
		return ready, err
	}
	nextReady, err := HLSVODSegmentReady(HLSVODSegmentPath(outDir, index+1))
	if err != nil {
		return false, err
	}
	if !nextReady {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return HLSVODSegmentBytesReady(data), nil
}

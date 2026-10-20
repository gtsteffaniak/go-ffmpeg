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
// encode job to finish segment index before serving.
func HLSVODSegmentReady(path string) (bool, error) {
	return hlsVODSegmentReadyAt(path)
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
	ready, err := hlsVODSegmentReadyAt(path)
	if err != nil || ready {
		return ready, err
	}
	nextReady, err := hlsVODSegmentReadyAt(HLSVODSegmentPath(outDir, index+1))
	if err != nil {
		return false, err
	}
	if !nextReady {
		return false, nil
	}
	return hlsVODSegmentReadyAt(path)
}

// hlsVODSegmentReadyAt checks publishability without reading whole segment mdat payloads.
func hlsVODSegmentReadyAt(path string) (bool, error) {
	if _, err := os.Stat(path + hlsVODSegmentTmpSuffix); err == nil {
		return false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return false, err
	}
	if st.Size() < int64(minHLSSegmentMediaBytes) {
		return false, nil
	}
	return mp4.PublishableHLSSegmentReaderAt(f, st.Size()), nil
}

// HLSVODOpenReadySegment opens path when it is publishable. The caller must close the file.
// When ready is false the returned file is nil and err is nil for a missing or incomplete segment.
func HLSVODOpenReadySegment(path string) (f *os.File, ready bool, err error) {
	if _, err := os.Stat(path + hlsVODSegmentTmpSuffix); err == nil {
		return nil, false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	st, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, false, err
	}
	if st.Size() < int64(minHLSSegmentMediaBytes) {
		file.Close()
		return nil, false, nil
	}
	if !mp4.PublishableHLSSegmentReaderAt(file, st.Size()) {
		file.Close()
		return nil, false, nil
	}
	return file, true, nil
}

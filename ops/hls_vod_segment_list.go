package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HLSVODInternalSegmentListName is ffmpeg's live CSV segment list for explicit-cut copy jobs.
const HLSVODInternalSegmentListName = ".ffmpeg-vod-segments.csv"

func hlsVODSegmentListPath(outDir string) string {
	return filepath.Join(outDir, HLSVODInternalSegmentListName)
}

// hlsVODSegmentListIndices returns segment indices ffmpeg has finished (closed) in the live CSV list.
func hlsVODSegmentListIndices(outDir string) (map[int]bool, error) {
	path := hlsVODSegmentListPath(outDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make(map[int]bool)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) == 0 {
			continue
		}
		fileField := strings.TrimSpace(parts[len(parts)-1])
		fileField = strings.Trim(fileField, "'\"")
		var idx int
		if _, err := fmt.Sscanf(filepath.Base(fileField), "%05d.m4s", &idx); err != nil {
			continue
		}
		out[idx] = true
	}
	return out, nil
}

func hlsVODSegmentListHas(outDir string, index int) (bool, error) {
	indices, err := hlsVODSegmentListIndices(outDir)
	if err != nil {
		return false, err
	}
	return indices[index], nil
}

// hlsVODSegmentListHead returns the smallest segment index not yet listed as complete.
func hlsVODSegmentListHead(outDir string, jobStart int) (int, error) {
	indices, err := hlsVODSegmentListIndices(outDir)
	if err != nil {
		return jobStart, err
	}
	head := jobStart
	for indices[head] {
		head++
	}
	return head, nil
}

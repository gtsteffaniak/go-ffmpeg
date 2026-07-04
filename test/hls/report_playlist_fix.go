package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/ops"
)

// finalizeContinuousOutputDir rewrites ffmpeg.m3u8 segment URIs for browser playback.
// Validation tolerates bare filenames via resolveContinuousSegmentPath; hls.js does not.
func finalizeContinuousOutputDir(outDir string) error {
	playlist := filepath.Join(outDir, "ffmpeg.m3u8")
	if err := ops.FixContinuousPlaylistTargetDuration(playlist); err != nil {
		return err
	}
	return ops.FixContinuousPlaylistSegmentURIs(outDir, playlist)
}

func fixReportContinuousPlaylists(reportDir string) (int, error) {
	mediaRoot := filepath.Join(reportDir, "media")
	st, err := os.Stat(mediaRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if !st.IsDir() {
		return 0, fmt.Errorf("%s is not a directory", mediaRoot)
	}

	fixed := 0
	err = filepath.WalkDir(mediaRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || d.Name() != "ffmpeg.m3u8" {
			return nil
		}
		outDir := filepath.Dir(path)
		if !strings.Contains(outDir, "continuous") {
			return nil
		}
		if err := finalizeContinuousOutputDir(outDir); err != nil {
			return err
		}
		fixed++
		return nil
	})
	return fixed, err
}

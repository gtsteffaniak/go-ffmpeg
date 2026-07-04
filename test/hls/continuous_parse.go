package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

type parsedContinuousPlaylist struct {
	InitURI   string
	Segments  []parsedPlaylistSegment
	HasEndList bool
	TargetDur  float64
}

type parsedPlaylistSegment struct {
	Index  int
	DurSec float64
	URI    string
}

func parseFFmpegM3U8(path string) (*parsedContinuousPlaylist, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := &parsedContinuousPlaylist{}
	scanner := bufio.NewScanner(f)
	var pendingDur float64
	index := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#EXTM3U") {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			raw := strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:")
			out.TargetDur, _ = strconv.ParseFloat(raw, 64)
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-MAP:") {
			out.InitURI = parsePlaylistURI(line)
			continue
		}
		if line == "#EXT-X-ENDLIST" {
			out.HasEndList = true
			continue
		}
		if strings.HasPrefix(line, "#EXTINF:") {
			raw := strings.TrimPrefix(line, "#EXTINF:")
			if i := strings.Index(raw, ","); i >= 0 {
				raw = raw[:i]
			}
			pendingDur, _ = strconv.ParseFloat(raw, 64)
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		out.Segments = append(out.Segments, parsedPlaylistSegment{
			Index:  index,
			DurSec: pendingDur,
			URI:    line,
		})
		index++
		pendingDur = 0
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parsePlaylistURI(line string) string {
	const prefix = `URI="`
	i := strings.Index(line, prefix)
	if i < 0 {
		return ""
	}
	rest := line[i+len(prefix):]
	if j := strings.Index(rest, `"`); j >= 0 {
		return rest[:j]
	}
	return rest
}

func resolveContinuousSegmentPath(outputDir, uri string) string {
	if uri == "" {
		return ""
	}
	if filepath.IsAbs(uri) {
		return uri
	}
	clean := filepath.Clean(uri)
	direct := filepath.Join(outputDir, clean)
	if st, err := os.Stat(direct); err == nil && !st.IsDir() {
		return direct
	}
	return filepath.Join(outputDir, "seg", filepath.Base(clean))
}

func resolveContinuousInitPath(outputDir, uri string) string {
	if uri == "" {
		return filepath.Join(outputDir, "init.m4s")
	}
	if filepath.IsAbs(uri) {
		return uri
	}
	return filepath.Join(outputDir, filepath.Clean(uri))
}

func validateInitTracks(init []byte, sourceHasAudio bool) []string {
	var issues []string
	if len(init) < 100 {
		issues = append(issues, fmt.Sprintf("init too small (%d bytes)", len(init)))
		return issues
	}
	tracks := mp4.TrackTimescalesFromInit(init)
	if len(tracks) < 1 {
		issues = append(issues, "init missing video track timescale")
	}
	if sourceHasAudio && len(tracks) < 2 {
		issues = append(issues, fmt.Sprintf("init has %d track(s), expected audio+video", len(tracks)))
	}
	return issues
}

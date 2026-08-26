package ops

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

// ContinuousValidationResult summarizes continuous HLS output checks.
type ContinuousValidationResult struct {
	SegmentCount int
	HasEndList   bool
	Issues       []mp4.TimelineIssue
}

// ValidateContinuousHLSOutput checks ffmpeg.m3u8 and segment files under outDir.
// For transcode jobs (copyts), segment durations may differ from the nominal grid;
// playlist #EXTINF values are treated as authoritative when present.
func ValidateContinuousHLSOutput(outDir string, opts HLSContinuousOptions, sourceHasAudio bool, toleranceSec float64) (ContinuousValidationResult, error) {
	if toleranceSec <= 0 {
		toleranceSec = mp4.DefaultHLSTimeToleranceSec
	}
	result := ContinuousValidationResult{}
	playlistPath := filepath.Join(outDir, "ffmpeg.m3u8")
	playlist, err := parseContinuousPlaylist(playlistPath)
	if err != nil {
		return result, err
	}
	result.HasEndList = playlist.HasEndList
	if !playlist.HasEndList {
		result.Issues = append(result.Issues, mp4.TimelineIssue{Check: "playlist", Message: "missing #EXT-X-ENDLIST"})
	}

	initPath := resolveContinuousMediaPath(outDir, playlist.InitURI, "init.m4s")
	initBytes, err := os.ReadFile(initPath)
	if err != nil {
		result.Issues = append(result.Issues, mp4.TimelineIssue{Check: "init", Message: err.Error()})
	} else if audioIssues := validateContinuousInitTracks(initBytes, sourceHasAudio); len(audioIssues) > 0 {
		for _, msg := range audioIssues {
			result.Issues = append(result.Issues, mp4.TimelineIssue{Check: "audio", Message: msg})
		}
	}

	trackTimescales := mp4.TrackTimescalesFromInit(initBytes)
	transcode := hlsContinuousUsesCopyTimestamps(opts)
	var prevStart, prevDur float64
	var prevIndex int
	for i, plSeg := range playlist.Segments {
		segPath := resolveContinuousMediaPath(outDir, plSeg.URI, "")
		media, err := os.ReadFile(segPath)
		if err != nil {
			result.Issues = append(result.Issues, mp4.TimelineIssue{
				Check:   "playlist_file",
				Message: fmt.Sprintf("segment %d missing on disk: %s", i, segPath),
			})
			continue
		}
		absIndex := opts.StartIndex + i
		expectedStart := mediaStartForSegmentIndex(opts, absIndex)
		expectedDur := opts.SegmentSec
		if plSeg.DurSec > 0 {
			expectedDur = plSeg.DurSec
		}
		if transcode {
			expectedStart = float64(absIndex) * opts.SegmentSec
		}
		startSec, _ := mp4.FragmentMediaStartSec(media)
		actualDur := mp4.FragmentDurationSecWithTimescales(media, trackTimescales)
		if actualDur <= 0 {
			actualDur = mp4.FragmentDurationSec(media)
		}
		if actualDur <= 0 && plSeg.DurSec > 0 {
			actualDur = plSeg.DurSec
		}

		checkMedia := media
		checkStart := startSec
		if transcode {
			aligned, alignErr := mp4.AlignFragmentToMediaStartWithTimescales(media, expectedStart, trackTimescales)
			if alignErr != nil {
				result.Issues = append(result.Issues, mp4.TimelineIssue{
					Check:   "align",
					Message: fmt.Sprintf("segment %d align to %.3fs: %v", absIndex, expectedStart, alignErr),
				})
			} else {
				checkMedia = aligned
				checkStart, _ = mp4.FragmentMediaStartSec(aligned)
				if dur := mp4.FragmentDurationSecWithTimescales(aligned, trackTimescales); dur > 0 {
					actualDur = dur
				}
			}
		}

		issues := mp4.ValidateSegmentTimeline(checkMedia, mp4.SegmentTimeline{
			Index:            absIndex,
			ExpectedStartSec: expectedStart,
			ExpectedDurSec:   expectedDur,
			MediaStartSec:    checkStart,
			ActualDurSec:     actualDur,
			Bytes:            len(checkMedia),
		}, toleranceSec)
		if transcode && plSeg.DurSec > 0 {
			issues = filterTranscodeDurationIssues(issues)
		}
		if i > 0 {
			prevTL := mp4.SegmentTimeline{Index: prevIndex, MediaStartSec: prevStart, ActualDurSec: prevDur}
			nextTL := mp4.SegmentTimeline{Index: absIndex, MediaStartSec: checkStart}
			issues = append(issues, mp4.ValidateContinuity(prevTL, nextTL, toleranceSec)...)
		}
		result.Issues = append(result.Issues, issues...)
		prevStart = checkStart
		prevDur = actualDur
		prevIndex = absIndex
	}
	result.SegmentCount = len(playlist.Segments)
	return result, nil
}

func filterTranscodeDurationIssues(issues []mp4.TimelineIssue) []mp4.TimelineIssue {
	if len(issues) == 0 {
		return issues
	}
	out := issues[:0]
	for _, issue := range issues {
		if issue.Check == "duration_match" {
			continue
		}
		out = append(out, issue)
	}
	return out
}

type continuousPlaylist struct {
	InitURI    string
	Segments   []continuousPlaylistSegment
	HasEndList bool
	TargetDur  float64
}

type continuousPlaylistSegment struct {
	DurSec float64
	URI    string
}

func parseContinuousPlaylist(path string) (*continuousPlaylist, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := &continuousPlaylist{}
	scanner := bufio.NewScanner(f)
	var pendingDur float64
	hasExtInf := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#EXTM3U") {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			raw := strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:")
			dur, parseErr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
			if parseErr != nil || dur <= 0 {
				return nil, fmt.Errorf("invalid #EXT-X-TARGETDURATION %q", raw)
			}
			out.TargetDur = dur
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-MAP:") {
			out.InitURI = parseContinuousPlaylistURI(line)
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
			dur, parseErr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
			if parseErr != nil || dur < 0 {
				return nil, fmt.Errorf("invalid #EXTINF %q", raw)
			}
			pendingDur = dur
			hasExtInf = true
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if !hasExtInf {
			return nil, fmt.Errorf("segment URI %q is missing a preceding #EXTINF", line)
		}
		out.Segments = append(out.Segments, continuousPlaylistSegment{DurSec: pendingDur, URI: line})
		pendingDur = 0
		hasExtInf = false
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseContinuousPlaylistURI(line string) string {
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

func resolveContinuousMediaPath(outputDir, uri, fallback string) string {
	if uri == "" {
		return filepath.Join(outputDir, fallback)
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

func validateContinuousInitTracks(init []byte, sourceHasAudio bool) []string {
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

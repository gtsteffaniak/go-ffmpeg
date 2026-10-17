package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

var continuousAlignLocks sync.Map // outDir -> *sync.Mutex

func lockContinuousAlign(outDir string) func() {
	v, _ := continuousAlignLocks.LoadOrStore(outDir, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

const (
	continuousAlignPollInterval = 50 * time.Millisecond
	continuousAlignTmpSuffix    = ".align.tmp"
	continuousRawSegmentDir     = "raw"
	continuousServePlaylistName = "index.m3u8"
	continuousJobStatusName     = "job.json"
)

// ContinuousJobStatus is the published state of a continuous HLS generation,
// written atomically to job.json whenever a segment is published or the job
// finishes. Consumers read it instead of parsing playlists for coverage.
type ContinuousJobStatus struct {
	// StartSec is the generation's effective media start — the snapped keyframe
	// for stream-copy seeks, the requested position otherwise.
	StartSec float64 `json:"startSec"`
	// FirstIndex is the lowest listed segment index; -1 when nothing is listed.
	FirstIndex int `json:"firstIndex"`
	// LastReadyIndex is the highest published segment index; -1 when nothing is.
	LastReadyIndex int `json:"lastReadyIndex"`
	// ProducedSec is the servable media duration (sum of published #EXTINF).
	ProducedSec float64 `json:"producedSec"`
	// EndList reports whether the source playlist reached #EXT-X-ENDLIST.
	EndList bool `json:"endList"`
	// Complete reports a finished generation: ENDLIST and every listed segment published.
	Complete bool `json:"complete"`
}

// ReadContinuousJobStatus loads job.json from a generation directory. A
// missing file yields an error — treat the generation as not ready.
func ReadContinuousJobStatus(outDir string) (ContinuousJobStatus, error) {
	var status ContinuousJobStatus
	data, err := os.ReadFile(filepath.Join(outDir, continuousJobStatusName))
	if err != nil {
		return status, err
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return status, err
	}
	return status, nil
}

// AlignContinuousSegmentFile rebases fMP4 decode timestamps to mediaStartSec on the HLS timeline.
// Init-segment timescales are loaded from init.m4s next to the segment or in the job directory.
func AlignContinuousSegmentFile(path string, mediaStartSec float64) error {
	return alignContinuousSegmentFile(path, mediaStartSec, trackTimescalesForSegmentFile(path))
}

func alignContinuousSegmentFile(path string, mediaStartSec float64, trackTimescales map[uint32]uint32) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	aligned, err := alignedContinuousSegment(data, mediaStartSec, trackTimescales)
	if err != nil {
		return err
	}
	return writeContinuousFileAtomic(path, aligned)
}

func alignedContinuousSegment(data []byte, mediaStartSec float64, trackTimescales map[uint32]uint32) ([]byte, error) {
	if !mp4.IsHLSSegmentFragment(data) {
		return nil, fmt.Errorf("segment not a parseable fMP4 fragment (%d bytes)", len(data))
	}
	return mp4.AlignFragmentToMediaStartWithTimescales(data, mediaStartSec, trackTimescales)
}

// alignContinuousSegmentInto validates and rebases the raw muxer output at
// rawPath, publishes it at dstPath via atomic rename, and removes the raw
// file — the published copy is the only fragment copy kept on disk.
func alignContinuousSegmentInto(rawPath, dstPath string, mediaStartSec float64, trackTimescales map[uint32]uint32) error {
	data, err := os.ReadFile(rawPath)
	if err != nil {
		return err
	}
	aligned, err := alignedContinuousSegment(data, mediaStartSec, trackTimescales)
	if err != nil {
		return err
	}
	if err := writeContinuousFileAtomic(dstPath, aligned); err != nil {
		return err
	}
	return os.Remove(rawPath)
}

func trackTimescalesForSegmentFile(path string) map[uint32]uint32 {
	dir := filepath.Dir(path)
	for _, candidate := range []string{
		filepath.Join(dir, "init.m4s"),
		filepath.Join(filepath.Dir(dir), "init.m4s"),
	} {
		if ts := loadContinuousInitTimescales(candidate); len(ts) > 0 {
			return ts
		}
	}
	return nil
}

func loadContinuousInitTimescales(initPath string) map[uint32]uint32 {
	data, err := os.ReadFile(initPath)
	if err != nil || len(data) == 0 {
		return nil
	}
	return mp4.TrackTimescalesFromInit(data)
}

// continuousPlaylistMediaStarts returns each listed segment's start on the
// generation-local playlist axis (cumulative #EXTINF from the first listed
// segment). Use continuousPlaylistAbsoluteMediaStarts when rebasing fMP4 tfdt
// for browser MSE on the full-file media timeline.
func continuousPlaylistMediaStarts(playlist *continuousPlaylist) map[int]float64 {
	if playlist == nil {
		return nil
	}
	starts := make(map[int]float64, len(playlist.Segments))
	var pos float64
	for _, seg := range playlist.Segments {
		if index, ok := continuousSegmentIndexFromURI(seg.URI); ok {
			starts[index] = pos
		}
		pos += seg.DurSec
	}
	return starts
}

// continuousPlaylistAbsoluteMediaStarts maps each segment index to its decode
// time on the full media timeline (0 at file head). Seek generations add
// generationStartSec so <video>.currentTime and hls.js buffer timestamps match
// the probed asset without client-side offset math.
func continuousPlaylistAbsoluteMediaStarts(playlist *continuousPlaylist, generationStartSec float64) map[int]float64 {
	relative := continuousPlaylistMediaStarts(playlist)
	if generationStartSec <= 0 || len(relative) == 0 {
		return relative
	}
	absolute := make(map[int]float64, len(relative))
	for index, pos := range relative {
		absolute[index] = generationStartSec + pos
	}
	return absolute
}

func continuousSegmentIndexFromURI(uri string) (int, bool) {
	name := filepath.Base(strings.TrimSpace(uri))
	if !continuousSegmentNamePattern.MatchString(name) {
		return 0, false
	}
	index, err := strconv.Atoi(strings.TrimSuffix(name, ".m4s"))
	return index, err == nil
}

func continuousRawSegmentPath(outDir string, index int) string {
	return filepath.Join(outDir, continuousRawSegmentDir, fmt.Sprintf("%05d.m4s", index))
}

func continuousSegmentPath(outDir string, index int) string {
	return filepath.Join(outDir, "seg", fmt.Sprintf("%05d.m4s", index))
}

// continuousSegmentPublished reports whether the segment was already aligned
// into the servable seg/ directory.
func continuousSegmentPublished(outDir string, index int) bool {
	path := continuousSegmentPath(outDir, index)
	st, err := os.Stat(path)
	if err != nil || st.Size() <= 0 {
		return false
	}
	_, err = os.Stat(path + continuousAlignTmpSuffix)
	return err != nil && os.IsNotExist(err)
}

// continuousRawSegmentComplete reports whether the raw muxer output for a
// playlist-listed segment is finished: closed by the muxer (no .tmp sibling)
// and large enough to be a parseable fMP4 fragment. The muxer only lists
// segments it has closed, so a listed raw file that survives these checks is
// safe to align — including short EOF tails under 8 KiB.
func continuousRawSegmentComplete(outDir string, index int) bool {
	path := continuousRawSegmentPath(outDir, index)
	st, err := os.Stat(path)
	if err != nil || st.Size() <= 0 {
		return false
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return mp4.IsHLSSegmentFragment(data)
}

func hlsContinuousUsesCopyTimestamps(opts HLSContinuousOptions) bool {
	return !opts.Remux && !opts.VideoCopy
}

// publishContinuousReadySegments aligns every complete raw segment the muxer
// playlist lists, publishes the contiguous prefix into index.m3u8, and writes
// job.json. Returns whether the published set changed.
func publishContinuousReadySegments(outDir string, startSec float64, published map[int]struct{}, jobDone bool) (bool, error) {
	unlock := lockContinuousAlign(outDir)
	defer unlock()

	playlist, err := parseContinuousPlaylist(filepath.Join(outDir, "ffmpeg.m3u8"))
	if err != nil {
		// A torn read of an append_list playlist is transient — retry next pass.
		// A parse failure on the final pass means the playlist is corrupt.
		if os.IsNotExist(err) || !jobDone {
			return false, nil
		}
		return false, err
	}
	if playlist == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Join(outDir, "seg"), 0o755); err != nil {
		return false, err
	}

	// hls.js places each fragment at its cumulative #EXTINF position — the
	// emitted playlist, not a predicted segment grid, is the timeline authority.
	starts := continuousPlaylistAbsoluteMediaStarts(playlist, startSec)
	timescales := loadContinuousInitTimescales(filepath.Join(outDir, "init.m4s"))
	changed := false
	for _, seg := range playlist.Segments {
		index, ok := continuousSegmentIndexFromURI(seg.URI)
		if !ok {
			continue
		}
		if _, done := published[index]; done {
			continue
		}
		if continuousSegmentPublished(outDir, index) {
			published[index] = struct{}{}
			changed = true
			continue
		}
		if !continuousRawSegmentComplete(outDir, index) {
			continue
		}
		// Transcode uses -copyts so ffmpeg may place tfdt on encoder/source time, and
		// stream-copy seeks can leave either track off the playlist grid; rebasing to
		// the emitted playlist position keeps MSE playback continuous without
		// #EXT-X-DISCONTINUITY.
		if err := alignContinuousSegmentInto(continuousRawSegmentPath(outDir, index), continuousSegmentPath(outDir, index), starts[index], timescales); err != nil {
			return false, err
		}
		published[index] = struct{}{}
		changed = true
	}
	if !changed && !jobDone {
		return false, nil
	}
	return changed, writeContinuousServeArtifacts(outDir, playlist, published, startSec)
}

// writeContinuousServeArtifacts emits index.m3u8 — the canonical serve
// playlist covering only the contiguous published prefix — and job.json.
func writeContinuousServeArtifacts(outDir string, playlist *continuousPlaylist, published map[int]struct{}, startSec float64) error {
	// Target duration covers every listed segment — published entries are a
	// prefix of them, so listed-but-unpublished durations count too.
	targetDur := playlist.TargetDur
	for _, seg := range playlist.Segments {
		if seg.DurSec > targetDur {
			targetDur = seg.DurSec
		}
	}
	status := ContinuousJobStatus{StartSec: startSec, FirstIndex: -1, LastReadyIndex: -1}
	var lines []string
	lines = append(lines, "#EXTM3U")
	emittedTarget := false
	for _, line := range playlist.Header {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#EXT-X-TARGETDURATION:") {
			continue
		}
		lines = append(lines, line)
		if !emittedTarget && strings.HasPrefix(trimmed, "#EXT-X-VERSION:") {
			lines = append(lines, "#EXT-X-TARGETDURATION:"+formatContinuousTargetDuration(targetDur))
			emittedTarget = true
		}
	}
	if !emittedTarget {
		lines = append(lines, "#EXT-X-TARGETDURATION:"+formatContinuousTargetDuration(targetDur))
	}

	emitted := 0
	for i, seg := range playlist.Segments {
		index, ok := continuousSegmentIndexFromURI(seg.URI)
		if !ok {
			continue
		}
		if i == 0 || status.FirstIndex < 0 {
			status.FirstIndex = index
		}
		if _, ok := published[index]; !ok {
			// The playlist is a contiguous prefix — a gap would hand hls.js a
			// fragment whose position it cannot compute.
			break
		}
		lines = append(lines, "#EXTINF:"+strconv.FormatFloat(seg.DurSec, 'f', -1, 64)+",")
		lines = append(lines, fmt.Sprintf("seg/%05d.m4s", index))
		status.LastReadyIndex = index
		status.ProducedSec += seg.DurSec
		emitted++
	}
	status.EndList = playlist.HasEndList
	status.Complete = playlist.HasEndList && emitted == len(playlist.Segments) && emitted > 0
	if status.Complete {
		lines = append(lines, "#EXT-X-ENDLIST")
	}
	if err := writeContinuousFileAtomic(filepath.Join(outDir, continuousServePlaylistName), []byte(strings.Join(lines, "\n")+"\n")); err != nil {
		return err
	}
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return writeContinuousFileAtomic(filepath.Join(outDir, continuousJobStatusName), data)
}

func formatContinuousTargetDuration(maxDur float64) string {
	if maxDur < 1 {
		maxDur = 1
	}
	return strconv.Itoa(int(math.Ceil(maxDur)))
}

// AlignContinuousHLSSegments rebases all segment files under outDir to the HLS
// media timeline and republishes the serve artifacts.
func AlignContinuousHLSSegments(outDir string, opts HLSContinuousOptions) error {
	if _, err := publishContinuousReadySegments(outDir, opts.StartSec, map[int]struct{}{}, true); err != nil {
		return err
	}
	return NormalizeContinuousPlaylist(outDir)
}

// FixContinuousPlaylistTargetDuration sets #EXT-X-TARGETDURATION to ceil(max #EXTINF).
func FixContinuousPlaylistTargetDuration(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines := strings.Split(string(data), "\n")
	maxDur := 0.0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#EXTINF:") {
			continue
		}
		raw := strings.TrimPrefix(line, "#EXTINF:")
		if i := strings.Index(raw, ","); i >= 0 {
			raw = raw[:i]
		}
		dur, parseErr := strconv.ParseFloat(raw, 64)
		if parseErr == nil && dur > maxDur {
			maxDur = dur
		}
	}
	if maxDur <= 0 {
		return nil
	}
	target := int(math.Ceil(maxDur))
	updated := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#EXT-X-TARGETDURATION:") {
			want := fmt.Sprintf("#EXT-X-TARGETDURATION:%d", target)
			if strings.TrimSpace(line) != want {
				lines[i] = want
				updated = true
			}
			break
		}
	}
	if !updated {
		return nil
	}
	return writeContinuousFileAtomic(path, []byte(strings.Join(lines, "\n")))
}

// FixContinuousPlaylistSegmentURIs rewrites bare segment filenames to seg/NNNNN.m4s when
// ffmpeg -f hls writes media under seg/ but lists 00000.m4s in the playlist.
func FixContinuousPlaylistSegmentURIs(outDir, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines := strings.Split(string(data), "\n")
	updated := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.Contains(trim, "/") {
			continue
		}
		direct := filepath.Join(outDir, trim)
		if st, statErr := os.Stat(direct); statErr == nil && !st.IsDir() {
			continue
		}
		segPath := filepath.Join(outDir, "seg", filepath.Base(trim))
		if st, statErr := os.Stat(segPath); statErr != nil || st.IsDir() {
			continue
		}
		lines[i] = filepath.ToSlash(filepath.Join("seg", filepath.Base(trim)))
		updated = true
	}
	if !updated {
		return nil
	}
	return writeContinuousFileAtomic(path, []byte(strings.Join(lines, "\n")))
}

func runContinuousSegmentAligner(parent context.Context, outDir string, job *HLSContinuousJob) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	published := make(map[int]struct{})
	ticker := time.NewTicker(continuousAlignPollInterval)
	defer ticker.Stop()

	go func() {
		if job != nil && job.ffmpegDone != nil {
			<-job.ffmpegDone
		}
		cancel()
		ticker.Stop()
		if _, err := publishContinuousReadySegments(outDir, job.startSec, published, true); err != nil {
			job.mu.Lock()
			if job.err == nil {
				job.err = err
			}
			job.mu.Unlock()
		}
		if err := NormalizeContinuousPlaylist(outDir); err != nil {
			job.mu.Lock()
			if job.err == nil {
				job.err = err
			}
			job.mu.Unlock()
		}
		if job != nil && job.alignDone != nil {
			close(job.alignDone)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = publishContinuousReadySegments(outDir, job.startSec, published, false)
		}
	}
}

func writeContinuousFileAtomic(path string, data []byte) error {
	tmp := path + continuousAlignTmpSuffix
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

package ops

import (
	"bufio"
	"context"
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
)

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
	if len(data) < mp4.MinHLSSegmentMediaBytes {
		return fmt.Errorf("segment too small (%d bytes)", len(data))
	}
	aligned, err := mp4.AlignFragmentToMediaStartWithTimescales(data, mediaStartSec, trackTimescales)
	if err != nil {
		return err
	}
	if err := writeContinuousFileAtomic(path, aligned); err != nil {
		return err
	}
	return markContinuousSegmentReady(path)
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

func markContinuousSegmentReady(path string) error {
	return os.WriteFile(path+".ready", []byte("1"), 0o644)
}

func continuousSegmentAligned(path string) bool {
	_, err := os.Stat(path + ".ready")
	return err == nil
}

func mediaStartForSegmentIndex(opts HLSContinuousOptions, index int) float64 {
	if len(opts.SegmentDurations) > 0 {
		var sum float64
		for i := 0; i < index && i < len(opts.SegmentDurations); i++ {
			if opts.SegmentDurations[i] > 0 {
				sum += opts.SegmentDurations[i]
			}
		}
		return sum
	}
	if index < len(opts.SegmentMediaStarts) {
		return opts.SegmentMediaStarts[index]
	}
	return float64(index) * opts.SegmentSec
}

func continuousSegmentPath(outDir string, index int) string {
	return filepath.Join(outDir, "seg", fmt.Sprintf("%05d.m4s", index))
}

func continuousSegmentReady(outDir string, index int, jobDone bool, opts HLSContinuousOptions) bool {
	path := continuousSegmentPath(outDir, index)
	st, err := os.Stat(path)
	if err != nil || st.Size() < int64(mp4.MinHLSSegmentMediaBytes) {
		return false
	}
	// hls_flags temp_file writes to .m4s.tmp and renames when complete; a present
	// .tmp means this index is still being muxed.
	if _, err := os.Stat(path + ".tmp"); err == nil {
		return false
	}
	if continuousPlaylistListsSegment(outDir, index) {
		return true
	}
	nextPath := continuousSegmentPath(outDir, index+1)
	if st, err := os.Stat(nextPath); err == nil && st.Size() > 0 {
		return true
	}
	if _, err := os.Stat(nextPath + ".tmp"); err == nil {
		return true
	}
	if jobDone {
		return true
	}
	return continuousPlaylistHasEndList(outDir)
}

func continuousPlaylistListsSegment(outDir string, index int) bool {
	data, err := os.ReadFile(filepath.Join(outDir, "ffmpeg.m3u8"))
	if err != nil {
		return false
	}
	text := string(data)
	for _, name := range []string{
		fmt.Sprintf("seg/%05d.m4s", index),
		fmt.Sprintf("%05d.m4s", index),
	} {
		if strings.Contains(text, name) {
			return true
		}
	}
	return false
}

func hlsContinuousUsesCopyTimestamps(opts HLSContinuousOptions) bool {
	return !opts.Remux && !opts.VideoCopy
}

func continuousPlaylistHasEndList(outDir string) bool {
	f, err := os.Open(filepath.Join(outDir, "ffmpeg.m3u8"))
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "#EXT-X-ENDLIST" {
			return true
		}
	}
	return false
}

func alignReadyContinuousSegments(outDir string, opts HLSContinuousOptions, aligned map[int]struct{}, jobDone bool) error {
	unlock := lockContinuousAlign(outDir)
	defer unlock()
	timescales := loadContinuousInitTimescales(filepath.Join(outDir, "init.m4s"))
	for index := opts.StartIndex; index < opts.StartIndex+4096; index++ {
		if _, ok := aligned[index]; ok {
			continue
		}
		if !continuousSegmentReady(outDir, index, jobDone, opts) {
			if index > opts.StartIndex && !continuousSegmentFileExists(outDir, index) {
				break
			}
			continue
		}
		mediaStart := mediaStartForSegmentIndex(opts, index)
		segPath := continuousSegmentPath(outDir, index)
		if continuousSegmentAligned(segPath) {
			aligned[index] = struct{}{}
			continue
		}
		// Transcode uses -copyts so ffmpeg may place tfdt on encoder/source time; rebasing
		// to the HLS media grid keeps MSE playback continuous without #EXT-X-DISCONTINUITY.
		err := alignContinuousSegmentFile(segPath, mediaStart, timescales)
		if err != nil {
			return err
		}
		aligned[index] = struct{}{}
	}
	return nil
}

func continuousSegmentFileExists(outDir string, index int) bool {
	st, err := os.Stat(continuousSegmentPath(outDir, index))
	return err == nil && st.Size() > 0
}

// AlignContinuousHLSSegments rebases all segment files under outDir to the HLS media timeline.
func AlignContinuousHLSSegments(outDir string, opts HLSContinuousOptions) error {
	aligned := make(map[int]struct{})
	jobDone := true
	if err := alignReadyContinuousSegments(outDir, opts, aligned, jobDone); err != nil {
		return err
	}
	if err := FixContinuousPlaylistTargetDuration(filepath.Join(outDir, "ffmpeg.m3u8")); err != nil {
		return err
	}
	return FixContinuousPlaylistSegmentURIs(outDir, filepath.Join(outDir, "ffmpeg.m3u8"))
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

func runContinuousSegmentAligner(parent context.Context, outDir string, opts HLSContinuousOptions, job *HLSContinuousJob) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	aligned := make(map[int]struct{})
	ticker := time.NewTicker(continuousAlignPollInterval)
	defer ticker.Stop()

	go func() {
		if job != nil && job.ffmpegDone != nil {
			<-job.ffmpegDone
		}
		cancel()
		ticker.Stop()
		if err := alignReadyContinuousSegments(outDir, opts, aligned, true); err != nil {
			job.mu.Lock()
			if job.err == nil {
				job.err = err
			}
			job.mu.Unlock()
		}
		_ = FixContinuousPlaylistTargetDuration(filepath.Join(outDir, "ffmpeg.m3u8"))
		_ = FixContinuousPlaylistSegmentURIs(outDir, filepath.Join(outDir, "ffmpeg.m3u8"))
		if job != nil && job.alignDone != nil {
			close(job.alignDone)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = alignReadyContinuousSegments(outDir, opts, aligned, false)
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

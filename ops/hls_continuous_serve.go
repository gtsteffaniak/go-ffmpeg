package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

var continuousSegmentNamePattern = regexp.MustCompile(`^[0-9]{5}\.m4s$`)

// NormalizeContinuousPlaylist applies canonical playlist fixes before serving.
// It sets #EXT-X-TARGETDURATION from #EXTINF values and rewrites bare segment
// filenames to seg/NNNNN.m4s when ffmpeg wrote media under seg/.
func NormalizeContinuousPlaylist(outDir string) error {
	playlist := filepath.Join(outDir, "ffmpeg.m3u8")
	if err := FixContinuousPlaylistTargetDuration(playlist); err != nil {
		return err
	}
	return FixContinuousPlaylistSegmentURIs(outDir, playlist)
}

// ContinuousInitReady reports whether init.m4s exists and is non-empty.
func ContinuousInitReady(outDir string) bool {
	path := filepath.Join(outDir, "init.m4s")
	st, err := os.Stat(path)
	return err == nil && st.Size() > 0
}

// ResolveContinuousSegmentPath resolves a segment name or playlist URI to a path
// under outDir using the same rules as continuous HLS validation.
func ResolveContinuousSegmentPath(outDir, name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if !continuousSegmentNamePattern.MatchString(name) {
		return "", fmt.Errorf("invalid continuous segment name %q", name)
	}
	path := resolveContinuousMediaPath(outDir, name, filepath.Join("seg", name))
	if err := assertContinuousPathContained(outDir, path); err != nil {
		return "", err
	}
	return path, nil
}

// ContinuousSegmentServeReady reports whether a segment is aligned and safe to serve.
func ContinuousSegmentServeReady(outDir, name string) bool {
	path, err := ResolveContinuousSegmentPath(outDir, name)
	if err != nil {
		return false
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() < int64(mp4.MinHLSSegmentMediaBytes) {
		return false
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		return false
	}
	return continuousSegmentAligned(path)
}

func assertContinuousPathContained(base, target string) error {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil || rel == ".." || hasParentPathPrefix(rel) {
		return fmt.Errorf("path escapes output dir")
	}
	return nil
}

func hasParentPathPrefix(rel string) bool {
	return len(rel) >= 3 && rel[:3] == ".."+string(os.PathSeparator)
}

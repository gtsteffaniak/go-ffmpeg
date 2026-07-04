package ops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

func TestAlignContinuousSegmentFileRebasesTimeline(t *testing.T) {
	t.Parallel()
	media := buildTestFragmentWithTFDT(64000) // 64000/90000 ≈ 0.711s
	dir := t.TempDir()
	path := filepath.Join(dir, "00001.m4s")
	if err := os.WriteFile(path, media, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AlignContinuousSegmentFile(path, 4.167); err != nil {
		t.Fatalf("AlignContinuousSegmentFile: %v", err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	start, err := mp4.FragmentMediaStartSec(out)
	if err != nil {
		t.Fatal(err)
	}
	const tol = 0.02
	if start < 4.167-tol || start > 4.167+tol {
		t.Fatalf("media start = %.3f, want ~4.167", start)
	}
}

func TestContinuousSegmentReadyFromPlaylist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	media := buildTestFragmentWithTFDT(64000)
	segPath := filepath.Join(dir, "seg", "00001.m4s")
	if err := os.WriteFile(segPath, media, 0o644); err != nil {
		t.Fatal(err)
	}
	playlist := "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4.0,\nseg/00001.m4s\n"
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg.m3u8"), []byte(playlist), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := HLSContinuousOptions{Remux: true}
	if !continuousSegmentReady(dir, 1, false, opts) {
		t.Fatal("expected segment ready when listed in ffmpeg playlist")
	}
}

func TestContinuousSegmentReadyTranscodeWithoutLargeNextSegment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	seg1 := buildTestFragmentWithTFDT(64000)
	if err := os.WriteFile(filepath.Join(dir, "seg", "00001.m4s"), seg1, 0o644); err != nil {
		t.Fatal(err)
	}
	// Next segment exists but is smaller than MinHLSSegmentMediaBytes (still muxing or short GOP).
	if err := os.WriteFile(filepath.Join(dir, "seg", "00002.m4s"), []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := HLSContinuousOptions{Remux: false, VideoCopy: false}
	if !continuousSegmentReady(dir, 1, false, opts) {
		t.Fatal("expected transcode segment ready without requiring next segment min size")
	}
}

func buildTestFragmentWithTFDT(ticks uint32) []byte {
	tfdt := makeFullAtom("tfdt", 0, uint32ToBytes(ticks))
	tfhd := makeFullAtom("tfhd", 0, []byte{0, 0, 0, 0, 0, 0, 0, 1})
	trafPayload := append(tfhd, tfdt...)
	traf := makeAtom("traf", trafPayload)
	moof := makeAtom("moof", traf)
	mdat := makeAtom("mdat", make([]byte, mp4.MinHLSSegmentMediaBytes-len(moof)-8))
	return append(moof, mdat...)
}

func makeAtom(typ string, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	putU32(out[0:4], uint32(8+len(payload)))
	copy(out[4:8], typ)
	copy(out[8:], payload)
	return out
}

func makeFullAtom(typ string, version byte, payload []byte) []byte {
	body := append([]byte{version, 0, 0, 0}, payload...)
	return makeAtom(typ, body)
}

func putU32(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

func uint32ToBytes(v uint32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

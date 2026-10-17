package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if err := AlignContinuousSegmentFile(path, 4.0); err != nil {
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
	if start < 4.0-tol || start > 4.0+tol {
		t.Fatalf("media start = %.3f, want ~4.0", start)
	}
}

func TestContinuousPlaylistMediaStartsCumulative(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	playlistText := "#EXTM3U\n#EXT-X-TARGETDURATION:14\n#EXT-X-MEDIA-SEQUENCE:384\n" +
		"#EXT-X-MAP:URI=\"init.m4s\"\n" +
		"#EXTINF:13.472,\nseg/00384.m4s\n" +
		"#EXTINF:5.005,\nseg/00385.m4s\n" +
		"#EXTINF:10.010,\nseg/00386.m4s\n"
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg.m3u8"), []byte(playlistText), 0o644); err != nil {
		t.Fatal(err)
	}
	playlist, err := parseContinuousPlaylist(filepath.Join(dir, "ffmpeg.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	starts := continuousPlaylistMediaStarts(playlist)
	want := map[int]float64{384: 0, 385: 13.472, 386: 18.477}
	for index, expected := range want {
		if got := starts[index]; got < expected-0.001 || got > expected+0.001 {
			t.Fatalf("index %d: got %v want %v", index, got, expected)
		}
	}
	if len(starts) != len(want) {
		t.Fatalf("got %d starts, want %d", len(starts), len(want))
	}
}

func TestPublishContinuousReadySegmentsUsesPlaylistTimeline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	// ffmpeg emitted GOP-boundary durations that no planned grid predicts; the
	// fragment must land on the playlist position, not a precomputed timeline.
	playlist := "#EXTM3U\n#EXT-X-TARGETDURATION:14\n#EXT-X-MEDIA-SEQUENCE:384\n" +
		"#EXT-X-MAP:URI=\"init.m4s\"\n" +
		"#EXTINF:13.472,\nraw/00384.m4s\n" +
		"#EXTINF:3.378,\nraw/00385.m4s\n"
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg.m3u8"), []byte(playlist), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{384, 385} {
		seg := buildTestFragmentWithTFDT(64000)
		if err := os.WriteFile(continuousRawSegmentPath(dir, index), seg, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := publishContinuousReadySegments(dir, 1887.0, map[int]struct{}{}, true); err != nil {
		t.Fatalf("publishContinuousReadySegments: %v", err)
	}
	cases := []struct {
		index int
		want  float64
	}{
		{384, 0},
		{385, 13.472},
	}
	for _, tc := range cases {
		out, err := os.ReadFile(filepath.Join(dir, "seg", fmt.Sprintf("%05d.m4s", tc.index)))
		if err != nil {
			t.Fatal(err)
		}
		start, err := mp4.FragmentMediaStartSec(out)
		if err != nil {
			t.Fatal(err)
		}
		const tol = 0.02
		if start < tc.want-tol || start > tc.want+tol {
			t.Fatalf("segment %d media start = %.3f, want ~%.3f", tc.index, start, tc.want)
		}
	}
	// The raw muxer output is consumed by publishing — only the aligned copy remains.
	for _, index := range []int{384, 385} {
		if _, err := os.Stat(continuousRawSegmentPath(dir, index)); !os.IsNotExist(err) {
			t.Fatalf("raw segment %d should be removed after publishing", index)
		}
	}
	status, err := ReadContinuousJobStatus(dir)
	if err != nil {
		t.Fatalf("ReadContinuousJobStatus: %v", err)
	}
	if status.FirstIndex != 384 || status.LastReadyIndex != 385 || status.StartSec != 1887.0 {
		t.Fatalf("job status = %+v", status)
	}
	if status.Complete {
		t.Fatal("job.json must not report complete without #EXT-X-ENDLIST")
	}
	index, err := os.ReadFile(filepath.Join(dir, "index.m3u8"))
	if err != nil {
		t.Fatalf("read index.m3u8: %v", err)
	}
	text := string(index)
	for _, want := range []string{"seg/00384.m4s", "seg/00385.m4s", "#EXT-X-MEDIA-SEQUENCE:384", `#EXT-X-MAP:URI="init.m4s"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("index.m3u8 missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "#EXT-X-ENDLIST") {
		t.Fatal("index.m3u8 must not end the playlist while the source lacks ENDLIST")
	}
}

func TestContinuousRawSegmentComplete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	if continuousRawSegmentComplete(dir, 1) {
		t.Fatal("missing raw file must not be complete")
	}
	media := buildTestFragmentWithTFDT(64000)
	rawPath := continuousRawSegmentPath(dir, 1)
	if err := os.WriteFile(rawPath, media, 0o644); err != nil {
		t.Fatal(err)
	}
	if !continuousRawSegmentComplete(dir, 1) {
		t.Fatal("expected complete raw segment")
	}
	if err := os.WriteFile(rawPath+".tmp", []byte("muxing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if continuousRawSegmentComplete(dir, 1) {
		t.Fatal("a .tmp sibling means the muxer is still writing")
	}
}

func TestContinuousSegmentPublished(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if continuousSegmentPublished(dir, 1) {
		t.Fatal("missing seg file must not be published")
	}
	segPath := filepath.Join(dir, "seg", "00001.m4s")
	if err := os.WriteFile(segPath, buildTestFragmentWithTFDT(64000), 0o644); err != nil {
		t.Fatal(err)
	}
	if !continuousSegmentPublished(dir, 1) {
		t.Fatal("expected published segment")
	}
	if err := os.WriteFile(segPath+continuousAlignTmpSuffix, []byte("in-progress"), 0o644); err != nil {
		t.Fatal(err)
	}
	if continuousSegmentPublished(dir, 1) {
		t.Fatal("an in-flight atomic rename must not read as published")
	}
}

func TestWriteContinuousFileAtomicUsesDistinctTmpSuffix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "00000.m4s")
	if err := writeContinuousFileAtomic(path, []byte("aligned")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Fatal("aligner must not write ffmpeg's .tmp suffix")
	}
	if _, err := os.Stat(path + continuousAlignTmpSuffix); err == nil {
		t.Fatal("align tmp should be renamed away")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "aligned" {
		t.Fatalf("got %q", got)
	}
}

func TestServePlaylistEmitsContiguousPublishedPrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, sub := range []string{"seg", "raw"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	playlist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:4\n#EXT-X-MEDIA-SEQUENCE:0\n" +
		"#EXT-X-MAP:URI=\"init.m4s\"\n" +
		"#EXTINF:4.0,\nraw/00000.m4s\n" +
		"#EXTINF:4.0,\nraw/00001.m4s\n" +
		"#EXTINF:4.0,\nraw/00002.m4s\n" +
		"#EXT-X-ENDLIST\n"
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg.m3u8"), []byte(playlist), 0o644); err != nil {
		t.Fatal(err)
	}
	// Only segments 0 and 1 are publishable; 2 is still muxing.
	for _, index := range []int{0, 1} {
		if err := os.WriteFile(continuousRawSegmentPath(dir, index), buildTestFragmentWithTFDT(64000), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := publishContinuousReadySegments(dir, 0, map[int]struct{}{}, false); err != nil {
		t.Fatalf("publishContinuousReadySegments: %v", err)
	}
	index, err := os.ReadFile(filepath.Join(dir, "index.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(index)
	if strings.Contains(text, "00002.m4s") || strings.Contains(text, "#EXT-X-ENDLIST") {
		t.Fatalf("serve playlist must withhold unlisted segments and ENDLIST:\n%s", text)
	}
	status, err := ReadContinuousJobStatus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !status.EndList || status.Complete {
		t.Fatalf("status = %+v, want endlist seen but generation incomplete", status)
	}
	// Once the tail publishes, ENDLIST lands on the serve playlist.
	if err := os.WriteFile(continuousRawSegmentPath(dir, 2), buildTestFragmentWithTFDT(64000), 0o644); err != nil {
		t.Fatal(err)
	}
	published := map[int]struct{}{0: {}, 1: {}}
	if _, err := publishContinuousReadySegments(dir, 0, published, true); err != nil {
		t.Fatalf("publishContinuousReadySegments final: %v", err)
	}
	index, err = os.ReadFile(filepath.Join(dir, "index.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "#EXT-X-ENDLIST") {
		t.Fatalf("complete generation must emit ENDLIST:\n%s", index)
	}
	status, err = ReadContinuousJobStatus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Complete || status.LastReadyIndex != 2 || status.ProducedSec < 11.9 {
		t.Fatalf("status = %+v, want complete", status)
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

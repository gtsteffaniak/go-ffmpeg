package mp4

import (
	"encoding/binary"
	"testing"
)

func testBox(typ string, payload []byte) []byte {
	b := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(b[:4], uint32(len(b)))
	copy(b[4:8], typ)
	copy(b[8:], payload)
	return b
}

func testInit() []byte {
	moov := testBox("moov", append(testBox("trak", nil), testBox("mvex", nil)...))
	return append(testBox("ftyp", make([]byte, 16)), moov...)
}

func TestInitSegmentComplete(t *testing.T) {
	if !InitSegmentComplete(testInit()) {
		t.Fatal("well-formed init segment rejected")
	}
}

func TestInitSegmentCompleteTruncated(t *testing.T) {
	init := testInit()
	for _, n := range []int{4, 20, 30, len(init) - 5} {
		if InitSegmentComplete(init[:n]) {
			t.Fatalf("truncated init (%d bytes) accepted", n)
		}
	}
}

func TestInitSegmentCompleteMissingBoxes(t *testing.T) {
	if InitSegmentComplete(testBox("moov", append(testBox("trak", nil), testBox("mvex", nil)...))) {
		t.Fatal("init without ftyp accepted")
	}
	if InitSegmentComplete(testBox("ftyp", make([]byte, 16))) {
		t.Fatal("init without moov accepted")
	}
	if InitSegmentComplete(append(testBox("ftyp", make([]byte, 16)), testBox("moov", testBox("trak", nil))...)) {
		t.Fatal("moov without mvex accepted")
	}
	if InitSegmentComplete(nil) || InitSegmentComplete([]byte{0, 0, 0}) {
		t.Fatal("empty/garbage init accepted")
	}
}

func TestInitSegmentCompleteTrailingGarbage(t *testing.T) {
	if InitSegmentComplete(append(testInit(), 1, 2, 3)) {
		t.Fatal("trailing partial box accepted")
	}
}

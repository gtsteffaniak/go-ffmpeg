package mp4

import (
	"encoding/binary"
	"testing"
)

func TestSplitInitMedia(t *testing.T) {
	t.Parallel()
	ftyp := makeAtom("ftyp", []byte{0, 0, 0, 1})
	moov := makeAtom("moov", []byte{1, 2, 3})
	moof := makeAtom("moof", []byte{4, 5, 6})
	mdat := makeAtom("mdat", []byte{7, 8, 9})
	data := append(append(append([]byte{}, ftyp...), moov...), append(moof, mdat...)...)

	init, media, err := SplitInitMedia(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(init) != len(ftyp)+len(moov) {
		t.Fatalf("init len = %d, want %d", len(init), len(ftyp)+len(moov))
	}
	if len(media) != len(moof)+len(mdat) {
		t.Fatalf("media len = %d, want %d", len(media), len(moof)+len(mdat))
	}
}

func makeAtom(typ string, payload []byte) []byte {
	size := 8 + len(payload)
	b := make([]byte, size)
	binary.BigEndian.PutUint32(b[0:4], uint32(size))
	copy(b[4:8], typ)
	copy(b[8:], payload)
	return b
}

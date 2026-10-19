package mp4

import (
	"os"
	"testing"
)

type readAtCounter struct {
	inner interface {
		ReadAt([]byte, int64) (int, error)
	}
	total int64
}

func (c *readAtCounter) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.inner.ReadAt(p, off)
	c.total += int64(n)
	return n, err
}

func TestPublishableHLSSegmentReaderAtSkipsMdat(t *testing.T) {
	t.Parallel()
	const timescale = defaultVideoTimescale
	trunPayload := []byte{0, 0, 0x01, 0x00}
	trunPayload = append(trunPayload, uint32ToBytes(2)...)
	trunPayload = append(trunPayload, uint32ToBytes(timescale)...)
	trunPayload = append(trunPayload, uint32ToBytes(timescale)...)
	trun := makeAtom("trun", trunPayload)
	tfhd := makeFullAtom("tfhd", 0, append([]byte{0, 0, 0, 0x01}, uint32ToBytes(1)...))
	tfdt := makeFullAtom("tfdt", 0, uint32ToBytes(0))
	trafPayload := append(tfhd, tfdt...)
	trafPayload = append(trafPayload, trun...)
	traf := makeAtom("traf", trafPayload)
	moof := makeAtom("moof", traf)
	mdat := makeAtom("mdat", make([]byte, 256*1024))
	media := append(moof, mdat...)

	dir := t.TempDir()
	path := dir + "/seg.m4s"
	if err := os.WriteFile(path, media, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	counter := &readAtCounter{inner: f}
	if !PublishableHLSSegmentReaderAt(counter, st.Size()) {
		t.Fatal("expected publishable segment")
	}
	if counter.total >= int64(len(media)/2) {
		t.Fatalf("read %d bytes of %d; expected only moof/header reads", counter.total, len(media))
	}
}

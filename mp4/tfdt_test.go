package mp4

import (
	"encoding/binary"
	"testing"
)

func TestOffsetDecodeTimeByStartSec(t *testing.T) {
	t.Parallel()
	tfhd := makeFullAtom("tfhd", 0, append([]byte{0, 0, 0, 0x01}, uint32ToBytes(1)...))
	tfdt := makeFullAtom("tfdt", 0, uint32ToBytes(0))
	trafPayload := append(tfhd, tfdt...)
	traf := makeAtom("traf", trafPayload)
	moof := makeAtom("moof", traf)
	mdat := makeAtom("mdat", []byte{1, 2, 3})
	media := append(moof, mdat...)

	out, err := OffsetDecodeTimeByStartSec(media, 4.0)
	if err != nil {
		t.Fatal(err)
	}
	got := readNestedTFDTv0(out)
	want := uint32(4.0 * defaultVideoTimescale)
	if got != want {
		t.Fatalf("tfdt = %d, want %d", got, want)
	}
}

func TestAlignFragmentToMediaStartFromZero(t *testing.T) {
	t.Parallel()
	media := sampleSingleMoofMedia(0)
	out, err := AlignFragmentToMediaStart(media, 8.0)
	if err != nil {
		t.Fatal(err)
	}
	got := readNestedTFDTv0(out)
	want := uint32(8.0 * defaultVideoTimescale)
	if got != want {
		t.Fatalf("tfdt = %d, want %d", got, want)
	}
}

func TestAlignFragmentToMediaStartRebasesRemux(t *testing.T) {
	t.Parallel()
	// Simulates stream copy placing first decode time at 110s while playlist expects 108s.
	wrongStart := uint32(110.0 * defaultVideoTimescale)
	media := sampleSingleMoofMedia(wrongStart)
	out, err := AlignFragmentToMediaStart(media, 108.0)
	if err != nil {
		t.Fatal(err)
	}
	got := readNestedTFDTv0(out)
	want := uint32(108.0 * defaultVideoTimescale)
	if got != want {
		t.Fatalf("tfdt = %d, want %d", got, want)
	}
}

func sampleSingleMoofMedia(firstTFDT uint32) []byte {
	tfhd := makeFullAtom("tfhd", 0, append([]byte{0, 0, 0, 0x01}, uint32ToBytes(1)...))
	tfdt := makeFullAtom("tfdt", 0, uint32ToBytes(firstTFDT))
	trafPayload := append(tfhd, tfdt...)
	traf := makeAtom("traf", trafPayload)
	moof := makeAtom("moof", traf)
	mdat := makeAtom("mdat", []byte{1, 2, 3})
	return append(moof, mdat...)
}

func readNestedTFDTv0(data []byte) uint32 {
	var found uint32
	_ = walkAtoms(data, 0, len(data), func(off, size int, typ string) error {
		if typ != "moof" {
			return nil
		}
		return walkAtoms(data, off+8, off+size, func(trafOff, trafSize int, trafTyp string) error {
			if trafTyp != "traf" {
				return nil
			}
			return walkAtoms(data, trafOff+8, trafOff+trafSize, func(atomOff, atomSize int, atomTyp string) error {
				if atomTyp != "tfdt" || atomOff+16 > len(data) {
					return nil
				}
				found = binary.BigEndian.Uint32(data[atomOff+12 : atomOff+16])
				return nil
			})
		})
	})
	return found
}

func readTFDTv0(data []byte) uint32 {
	off := 0
	for off+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[off : off+4]))
		typ := string(data[off+4 : off+8])
		if typ == "tfdt" {
			return binary.BigEndian.Uint32(data[off+12 : off+16])
		}
		off += size
	}
	return 0
}

func makeFullAtom(typ string, version byte, payload []byte) []byte {
	body := append([]byte{version, 0, 0, 0}, payload...)
	return makeAtom(typ, body)
}

func uint32ToBytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

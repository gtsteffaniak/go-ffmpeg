package mp4

import (
	"encoding/binary"
	"math"
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

func TestAlignFragmentPreservesAVOffset(t *testing.T) {
	t.Parallel()
	const videoTS = uint32(90000)
	const audioTS = uint32(48000)
	timescales := map[uint32]uint32{1: videoTS, 2: audioTS}

	videoTicks := uint32(2.0 * float64(videoTS))
	audioTicks := uint32(2.02 * float64(audioTS)) // 20ms audio lead
	media := sampleDualTrackMoofMedia(videoTicks, audioTicks)

	out, err := AlignFragmentToMediaStartWithTimescales(media, 8.0, timescales)
	if err != nil {
		t.Fatal(err)
	}
	videoStart, err := FragmentMediaStartSecForTrack(out, 1, timescales)
	if err != nil {
		t.Fatal(err)
	}
	audioStart, err := FragmentMediaStartSecForTrack(out, 2, timescales)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(videoStart-8.0) > 0.001 {
		t.Fatalf("video start = %.3f, want 8.0", videoStart)
	}
	wantOffset := float64(audioTicks)/float64(audioTS) - float64(videoTicks)/float64(videoTS)
	if math.Abs((audioStart-videoStart)-wantOffset) > 0.001 {
		t.Fatalf("A/V offset = %.3f, want %.3f", audioStart-videoStart, wantOffset)
	}
}

func TestAlignAndReadWithHLSFMP4Timescale(t *testing.T) {
	t.Parallel()
	const hlsVideoTS = uint32(12288)
	timescales := map[uint32]uint32{1: hlsVideoTS}
	media := sampleSingleMoofMedia(0)
	out, err := AlignFragmentToMediaStartWithTimescales(media, 4.0, timescales)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FragmentMediaStartSecWithTimescales(out, timescales)
	if err != nil {
		t.Fatal(err)
	}
	if got < 3.99 || got > 4.01 {
		t.Fatalf("aligned start = %.3f, want 4.000", got)
	}
	ticks := readNestedTFDTv0(out)
	if ticks != uint32(4.0*float64(hlsVideoTS)) {
		t.Fatalf("tfdt ticks = %d, want %d", ticks, uint32(4.0*float64(hlsVideoTS)))
	}
}

func sampleDualTrackMoofMedia(videoTFDT, audioTFDT uint32) []byte {
	videoTraf := makeAtom("traf", append(tfhdForTrack(1), makeFullAtom("tfdt", 0, uint32ToBytes(videoTFDT))...))
	audioTraf := makeAtom("traf", append(tfhdForTrack(2), makeFullAtom("tfdt", 0, uint32ToBytes(audioTFDT))...))
	moof := makeAtom("moof", append(videoTraf, audioTraf...))
	mdat := makeAtom("mdat", make([]byte, 64))
	return append(moof, mdat...)
}

func tfhdForTrack(trackID uint32) []byte {
	return makeFullAtom("tfhd", 0, uint32ToBytes(trackID))
}

func sampleSingleMoofMedia(firstTFDT uint32) []byte {
	tfhd := tfhdForTrack(1)
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

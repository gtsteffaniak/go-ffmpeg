package mp4

import "testing"

func TestFragmentDurationSec(t *testing.T) {
	t.Parallel()
	const timescale = defaultVideoTimescale
	// 2 samples of 90000 ticks each = 2 seconds total.
	trunPayload := []byte{0, 0, 0x01, 0x00} // version 0, sample-duration flag
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
	mdat := makeAtom("mdat", []byte{1, 2, 3})
	media := append(moof, mdat...)

	got := FragmentDurationSec(media)
	if got < 1.999 || got > 2.001 {
		t.Fatalf("FragmentDurationSec = %.3f, want 2.000", got)
	}
}

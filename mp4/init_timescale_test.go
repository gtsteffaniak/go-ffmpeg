package mp4

import "testing"

func TestFragmentDurationMinSecWithTimescales(t *testing.T) {
	t.Parallel()
	const videoTS = uint32(90000)
	const audioTS = uint32(48000)

	videoTrun := makeTrunAtom(videoTS, videoTS) // 2s video
	audioTrun := makeTrunAtom(36000, 36000)     // 1.5s audio at 48kHz

	videoTfhd := makeTfhdAtom(1)
	audioTfhd := makeTfhdAtom(2)
	videoTraf := makeAtom("traf", append(videoTfhd, videoTrun...))
	audioTraf := makeAtom("traf", append(audioTfhd, audioTrun...))
	moof := makeAtom("moof", append(videoTraf, audioTraf...))
	media := append(moof, makeAtom("mdat", []byte{1, 2, 3})...)

	timescales := map[uint32]uint32{1: videoTS, 2: audioTS}

	secs := fragmentTrackDurationSecs(media, timescales)
	if len(secs) < 2 {
		t.Fatalf("track durations = %v, want two tracks", secs)
	}

	maxDur := FragmentDurationSecWithTimescales(media, timescales)
	minDur := FragmentDurationMinSecWithTimescales(media, timescales)
	if minDur <= 0 || maxDur <= 0 {
		t.Fatalf("min=%.3f max=%.3f, want positive durations", minDur, maxDur)
	}
	if minDur >= maxDur {
		t.Fatalf("min duration %.3f must be less than max %.3f (check dual-track moof fixture)", minDur, maxDur)
	}
	if minDur < 1.499 || minDur > 1.501 {
		t.Fatalf("min duration = %.3f, want ~1.500", minDur)
	}
	if maxDur < 1.999 || maxDur > 2.001 {
		t.Fatalf("max duration = %.3f, want ~2.000", maxDur)
	}
}

func makeTrunAtom(dur1, dur2 uint32) []byte {
	trunPayload := []byte{0, 0, 0x01, 0x00}
	trunPayload = append(trunPayload, uint32ToBytes(2)...)
	trunPayload = append(trunPayload, uint32ToBytes(dur1)...)
	trunPayload = append(trunPayload, uint32ToBytes(dur2)...)
	return makeAtom("trun", trunPayload)
}

func makeTfhdAtom(trackID uint32) []byte {
	// version 0 + default-base-is-moof (0x020000) + track_ID — matches ffmpeg fMP4 tfhd layout.
	payload := append([]byte{0, 0x02, 0x00, 0x00}, uint32ToBytes(trackID)...)
	return makeAtom("tfhd", payload)
}

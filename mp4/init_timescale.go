package mp4

import "encoding/binary"

// TrackTimescalesFromInit reads per-track timescales from an fMP4 init (moov) segment.
func TrackTimescalesFromInit(init []byte) map[uint32]uint32 {
	out := make(map[uint32]uint32)
	_ = walkAtoms(init, 0, len(init), func(off, size int, typ string) error {
		if typ != "moov" {
			return nil
		}
		end := off + size
		return walkAtoms(init, off+8, end, func(trakOff, trakSize int, trakTyp string) error {
			if trakTyp != "trak" {
				return nil
			}
			trakEnd := trakOff + trakSize
			trackID, ok := tkhdTrackID(init, trakOff, trakEnd)
			if !ok {
				return nil
			}
			ts, ok := trakMDHDTimescale(init, trakOff, trakEnd)
			if !ok {
				return nil
			}
			out[trackID] = ts
			return nil
		})
	})
	return out
}

func tkhdTrackID(data []byte, trakOff, trakEnd int) (uint32, bool) {
	var trackID uint32
	found := false
	_ = walkAtoms(data, trakOff+8, trakEnd, func(atomOff, atomSize int, atomTyp string) error {
		if atomTyp != "tkhd" {
			return nil
		}
		// tkhd v0: track_id at byte 12 of full atom payload (version+flags+...)
		if atomOff+20 > len(data) {
			return nil
		}
		version := data[atomOff+8]
		idOff := atomOff + 20
		if version == 1 {
			idOff = atomOff + 32
		}
		if idOff+4 > atomOff+atomSize || idOff+4 > len(data) {
			return nil
		}
		trackID = binary.BigEndian.Uint32(data[idOff : idOff+4])
		found = true
		return nil
	})
	return trackID, found
}

func trakMDHDTimescale(data []byte, trakOff, trakEnd int) (uint32, bool) {
	var timescale uint32
	found := false
	_ = walkAtoms(data, trakOff+8, trakEnd, func(atomOff, atomSize int, atomTyp string) error {
		if atomTyp != "mdia" {
			return nil
		}
		mdiaEnd := atomOff + atomSize
		return walkAtoms(data, atomOff+8, mdiaEnd, func(mdiaChildOff, mdiaChildSize int, mdiaChildTyp string) error {
			if mdiaChildTyp != "mdhd" {
				return nil
			}
			ts, ok := mdhdTimescale(data, mdiaChildOff, mdiaChildSize)
			if ok {
				timescale = ts
				found = true
			}
			return nil
		})
	})
	return timescale, found
}

func mdhdTimescale(data []byte, atomOff, atomSize int) (uint32, bool) {
	if atomOff+16 > len(data) || atomSize < 16 {
		return 0, false
	}
	version := data[atomOff+8]
	switch version {
	case 0:
		if atomOff+24 > len(data) {
			return 0, false
		}
		return binary.BigEndian.Uint32(data[atomOff+20 : atomOff+24]), true
	case 1:
		if atomOff+32 > len(data) {
			return 0, false
		}
		return binary.BigEndian.Uint32(data[atomOff+28 : atomOff+32]), true
	default:
		return 0, false
	}
}

// FragmentDurationSecWithTimescales returns the longest track span using init timescales when known.
func FragmentDurationSecWithTimescales(media []byte, trackTimescales map[uint32]uint32) float64 {
	if len(media) == 0 {
		return 0
	}
	trackTicks := make(map[uint32]uint64)
	_ = walkAtoms(media, 0, len(media), func(off, size int, typ string) error {
		if typ != "moof" {
			return nil
		}
		end := off + size
		return walkAtoms(media, off+8, end, func(trafOff, trafSize int, trafTyp string) error {
			if trafTyp != "traf" {
				return nil
			}
			trafEnd := trafOff + trafSize
			trackID, err := tfhdTrackID(media, trafOff, trafEnd)
			if err != nil {
				return err
			}
			ticks, ok := trafDurationTicks(media, trafOff, trafEnd)
			if !ok || ticks == 0 {
				return nil
			}
			trackTicks[trackID] += ticks
			return nil
		})
	})
	var maxSec float64
	for trackID, ticks := range trackTicks {
		ts := trackTimescales[trackID]
		var sec float64
		if ts > 0 {
			sec = float64(ticks) / float64(ts)
		} else {
			sec = inferTrackDurationSec(ticks)
		}
		if sec > maxSec {
			maxSec = sec
		}
	}
	return maxSec
}

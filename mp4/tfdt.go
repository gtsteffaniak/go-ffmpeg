package mp4

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	defaultVideoTimescale = 90000
	defaultAudioTimescale = 48000
)

// OffsetDecodeTimeByStartSec shifts baseMediaDecodeTime in each traf tfdt atom by
// startSec on the track timescale. Prefer AlignFragmentToMediaStart when ffmpeg may
// already have placed fragments on a non-zero decode timeline (stream copy / remux).
func OffsetDecodeTimeByStartSec(media []byte, startSec float64) ([]byte, error) {
	if startSec <= 0 || len(media) == 0 {
		return media, nil
	}
	out := append([]byte(nil), media...)
	if err := patchTFDTs(out, startSec, true); err != nil {
		return nil, err
	}
	return out, nil
}

// AlignFragmentToMediaStart rebases each track so its earliest tfdt in the fragment
// equals mediaStartSec on the HLS playlist timeline. Works for transcode (tfdt≈0),
// remux/copy (source PTS + output_ts_offset), and mixed multi-moof segments.
func AlignFragmentToMediaStart(media []byte, mediaStartSec float64) ([]byte, error) {
	if len(media) == 0 {
		return media, nil
	}
	out := append([]byte(nil), media...)
	firstByTrack, err := firstTFDTByTrack(out)
	if err != nil {
		return nil, err
	}
	if len(firstByTrack) == 0 {
		return out, nil
	}
	deltaByTrack := make(map[uint32]int64, len(firstByTrack))
	for trackID, firstTicks := range firstByTrack {
		timescale := timescaleForTrack(trackID)
		target := int64(math.Round(mediaStartSec * float64(timescale)))
		deltaByTrack[trackID] = target - int64(firstTicks)
	}
	if err := applyTFDTDeltas(out, deltaByTrack); err != nil {
		return nil, err
	}
	return out, nil
}

func patchTFDTs(data []byte, startSec float64, additive bool) error {
	return walkAtoms(data, 0, len(data), func(off, size int, typ string) error {
		if typ != "moof" {
			return nil
		}
		end := off + size
		return walkAtoms(data, off+8, end, func(trafOff, trafSize int, trafTyp string) error {
			if trafTyp != "traf" {
				return nil
			}
			trafEnd := trafOff + trafSize
			trackID, err := tfhdTrackID(data, trafOff, trafEnd)
			if err != nil {
				return err
			}
			timescale := timescaleForTrack(trackID)
			offsetTicks := uint64(math.Round(startSec * float64(timescale)))
			return walkAtoms(data, trafOff+8, trafEnd, func(atomOff, atomSize int, atomTyp string) error {
				if atomTyp != "tfdt" {
					return nil
				}
				if additive {
					return addTFDTOffset(data, atomOff, atomSize, offsetTicks)
				}
				return setTFDTTicks(data, atomOff, atomSize, offsetTicks)
			})
		})
	})
}

func firstTFDTByTrack(data []byte) (map[uint32]uint64, error) {
	out := make(map[uint32]uint64)
	err := walkAtoms(data, 0, len(data), func(off, size int, typ string) error {
		if typ != "moof" {
			return nil
		}
		end := off + size
		return walkAtoms(data, off+8, end, func(trafOff, trafSize int, trafTyp string) error {
			if trafTyp != "traf" {
				return nil
			}
			trafEnd := trafOff + trafSize
			trackID, err := tfhdTrackID(data, trafOff, trafEnd)
			if err != nil {
				return err
			}
			if _, ok := out[trackID]; ok {
				return nil
			}
			return walkAtoms(data, trafOff+8, trafEnd, func(atomOff, atomSize int, atomTyp string) error {
				if atomTyp != "tfdt" {
					return nil
				}
				ticks, err := readTFDTTicks(data, atomOff, atomSize)
				if err != nil {
					return err
				}
				out[trackID] = ticks
				return nil
			})
		})
	})
	return out, err
}

func applyTFDTDeltas(data []byte, deltaByTrack map[uint32]int64) error {
	return walkAtoms(data, 0, len(data), func(off, size int, typ string) error {
		if typ != "moof" {
			return nil
		}
		end := off + size
		return walkAtoms(data, off+8, end, func(trafOff, trafSize int, trafTyp string) error {
			if trafTyp != "traf" {
				return nil
			}
			trafEnd := trafOff + trafSize
			trackID, err := tfhdTrackID(data, trafOff, trafEnd)
			if err != nil {
				return err
			}
			delta, ok := deltaByTrack[trackID]
			if !ok {
				return nil
			}
			return walkAtoms(data, trafOff+8, trafEnd, func(atomOff, atomSize int, atomTyp string) error {
				if atomTyp != "tfdt" {
					return nil
				}
				return addTFDTDelta(data, atomOff, atomSize, delta)
			})
		})
	})
}

func readTFDTTicks(data []byte, atomOff, atomSize int) (uint64, error) {
	if atomOff+16 > len(data) || atomSize < 16 {
		return 0, fmt.Errorf("tfdt too short")
	}
	version := data[atomOff+8]
	switch version {
	case 0:
		if atomOff+16 > len(data) {
			return 0, fmt.Errorf("tfdt v0 too short")
		}
		return uint64(binary.BigEndian.Uint32(data[atomOff+12 : atomOff+16])), nil
	case 1:
		if atomOff+20 > len(data) {
			return 0, fmt.Errorf("tfdt v1 too short")
		}
		return binary.BigEndian.Uint64(data[atomOff+12 : atomOff+20]), nil
	default:
		return 0, fmt.Errorf("unsupported tfdt version %d", version)
	}
}

func timescaleForTrack(trackID uint32) uint32 {
	switch trackID {
	case 1:
		return defaultVideoTimescale
	case 2:
		return defaultAudioTimescale
	default:
		return defaultVideoTimescale
	}
}

func tfhdTrackID(data []byte, trafOff, trafEnd int) (uint32, error) {
	var trackID uint32
	found := false
	err := walkAtoms(data, trafOff+8, trafEnd, func(atomOff, atomSize int, atomTyp string) error {
		if atomTyp != "tfhd" {
			return nil
		}
		if atomOff+16 > len(data) {
			return fmt.Errorf("tfhd too short")
		}
		trackID = binary.BigEndian.Uint32(data[atomOff+12 : atomOff+16])
		found = true
		return nil
	})
	if err != nil {
		return 0, err
	}
	if !found {
		return 1, nil
	}
	return trackID, nil
}

func addTFDTOffset(data []byte, atomOff, atomSize int, offsetTicks uint64) error {
	return addTFDTDelta(data, atomOff, atomSize, int64(offsetTicks))
}

func addTFDTDelta(data []byte, atomOff, atomSize int, delta int64) error {
	if atomOff+16 > len(data) || atomSize < 16 {
		return fmt.Errorf("tfdt too short")
	}
	version := data[atomOff+8]
	switch version {
	case 0:
		if atomOff+16 > len(data) {
			return fmt.Errorf("tfdt v0 too short")
		}
		cur := int64(binary.BigEndian.Uint32(data[atomOff+12 : atomOff+16]))
		sum := cur + delta
		if sum < 0 || sum > math.MaxUint32 {
			return fmt.Errorf("tfdt v0 overflow")
		}
		binary.BigEndian.PutUint32(data[atomOff+12:atomOff+16], uint32(sum))
	case 1:
		if atomOff+20 > len(data) {
			return fmt.Errorf("tfdt v1 too short")
		}
		cur := int64(binary.BigEndian.Uint64(data[atomOff+12 : atomOff+20]))
		sum := cur + delta
		if sum < 0 {
			return fmt.Errorf("tfdt v1 underflow")
		}
		binary.BigEndian.PutUint64(data[atomOff+12:atomOff+20], uint64(sum))
	default:
		return fmt.Errorf("unsupported tfdt version %d", version)
	}
	return nil
}

func setTFDTTicks(data []byte, atomOff, atomSize int, ticks uint64) error {
	if atomOff+16 > len(data) || atomSize < 16 {
		return fmt.Errorf("tfdt too short")
	}
	version := data[atomOff+8]
	switch version {
	case 0:
		if ticks > math.MaxUint32 {
			return fmt.Errorf("tfdt v0 overflow")
		}
		binary.BigEndian.PutUint32(data[atomOff+12:atomOff+16], uint32(ticks))
	case 1:
		binary.BigEndian.PutUint64(data[atomOff+12:atomOff+20], ticks)
	default:
		return fmt.Errorf("unsupported tfdt version %d", version)
	}
	return nil
}

func walkAtoms(data []byte, start, end int, fn func(off, size int, typ string) error) error {
	off := start
	for off+8 <= end && off+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[off : off+4]))
		if size < 8 {
			return fmt.Errorf("invalid atom size %d at %d", size, off)
		}
		typ := string(data[off+4 : off+8])
		if err := fn(off, size, typ); err != nil {
			return err
		}
		if size == 0 {
			break
		}
		off += size
	}
	return nil
}

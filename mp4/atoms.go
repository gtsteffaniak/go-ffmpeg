package mp4

import (
	"encoding/binary"
	"fmt"
)

// SplitInitMedia separates fMP4 init (ftyp+moov) from media (moof+mdat…) at the first moof atom.
func SplitInitMedia(data []byte) (init, media []byte, err error) {
	if len(data) < 8 {
		return nil, nil, fmt.Errorf("mp4 data too short")
	}
	off := 0
	for off+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[off : off+4]))
		if size < 8 {
			return nil, nil, fmt.Errorf("invalid mp4 atom size at offset %d", off)
		}
		typ := string(data[off+4 : off+8])
		if typ == "moof" {
			return data[:off], data[off:], nil
		}
		if size == 0 {
			break
		}
		off += size
	}
	return data, nil, nil
}

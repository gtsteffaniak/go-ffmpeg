package mp4

import (
	"encoding/binary"
	"io"
)

const maxHLSSegmentMoofBytes = 4 << 20

// PublishableHLSSegmentReaderAt reports whether an on-disk fMP4 media segment is
// publishable, reading only top-level moof boxes (mdat and other payloads are skipped).
func PublishableHLSSegmentReaderAt(r io.ReaderAt, size int64) bool {
	if r == nil || size < int64(MinHLSParseableFragmentBytes) {
		return false
	}
	moof, ok := readTopLevelMoofBoxes(r, size)
	if !ok || len(moof) < MinHLSParseableFragmentBytes {
		return false
	}
	return PublishableHLSSegmentFragment(moof)
}

// readTopLevelMoofBoxes concatenates top-level moof box bytes from a segment file.
func readTopLevelMoofBoxes(r io.ReaderAt, size int64) ([]byte, bool) {
	var moof []byte
	off := int64(0)
	var hdr [8]byte
	for off+8 <= size {
		if _, err := r.ReadAt(hdr[:], off); err != nil {
			return nil, false
		}
		boxSize := int64(binary.BigEndian.Uint32(hdr[0:4]))
		typ := string(hdr[4:8])
		if boxSize == 0 {
			boxSize = size - off
		}
		if boxSize < 8 {
			if boxSize == 1 {
				return nil, false
			}
			return nil, false
		}
		if off+boxSize > size {
			return nil, false
		}
		if typ == "moof" {
			if boxSize > maxHLSSegmentMoofBytes {
				return nil, false
			}
			buf := make([]byte, boxSize)
			if _, err := r.ReadAt(buf, off); err != nil {
				return nil, false
			}
			moof = append(moof, buf...)
		}
		off += boxSize
	}
	return moof, true
}

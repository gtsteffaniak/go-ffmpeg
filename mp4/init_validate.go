package mp4

import "encoding/binary"

type mp4Box struct {
	typ        string
	start, end int // payload bounds within data
}

// walkBoxes parses the flat box list in data, requiring every box to be
// complete: no truncated header, no box extending past the buffer. A size-0
// box (extends to EOF) is only valid as the final box.
func walkBoxes(data []byte) ([]mp4Box, bool) {
	var boxes []mp4Box
	off := 0
	for off < len(data) {
		if len(data)-off < 8 {
			return nil, false
		}
		size64 := uint64(binary.BigEndian.Uint32(data[off : off+4]))
		typ := string(data[off+4 : off+8])
		header := 8
		if size64 == 1 {
			if len(data)-off < 16 {
				return nil, false
			}
			size64 = binary.BigEndian.Uint64(data[off+8 : off+16])
			header = 16
		} else if size64 == 0 {
			size64 = uint64(len(data) - off)
		}
		end := off + int(size64)
		if size64 < uint64(header) || end > len(data) {
			return nil, false
		}
		boxes = append(boxes, mp4Box{typ: typ, start: off + header, end: end})
		off = end
	}
	return boxes, true
}

// InitSegmentComplete reports whether data is a structurally complete fMP4
// init segment: an ftyp/styp box plus a complete moov containing at least one
// trak and an mvex box. It exists so readers can distinguish a finished
// init.m4s from one ffmpeg is still writing.
func InitSegmentComplete(data []byte) bool {
	boxes, ok := walkBoxes(data)
	if !ok || len(boxes) == 0 {
		return false
	}
	haveType, moovOK := false, false
	for _, b := range boxes {
		switch b.typ {
		case "ftyp", "styp":
			haveType = true
		case "moov":
			moovOK = moovChildrenComplete(data[b.start:b.end])
		}
	}
	return haveType && moovOK
}

func moovChildrenComplete(moov []byte) bool {
	children, ok := walkBoxes(moov)
	if !ok {
		return false
	}
	var trak, mvex bool
	for _, c := range children {
		switch c.typ {
		case "trak":
			trak = true
		case "mvex":
			mvex = true
		}
	}
	return trak && mvex
}

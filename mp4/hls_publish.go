package mp4

// PublishableHLSSegmentFragment reports whether media is safe to serve as a completed
// HLS fMP4 segment (parseable moof/tfdt and monotonic per-track decode times).
func PublishableHLSSegmentFragment(media []byte) bool {
	if !IsHLSSegmentFragment(media) {
		return false
	}
	return validateMonotonicTFDT(media) == nil
}

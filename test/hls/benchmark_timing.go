package main

// computeSegmentTiming derives cold-start vs steady-state segment metrics from a segment run.
// Segment 0 pays init + first-process startup; segments 1+ approximate per-request cost
// after warm-up (matches production playlist warm of seg 0–2).
func computeSegmentTiming(segments []segmentBenchmark, mediaDurationSec float64) encodeTimingSummary {
	if len(segments) == 0 {
		return encodeTimingSummary{}
	}
	var warmTotal int64
	for i, s := range segments {
		if i == 0 {
			continue
		}
		warmTotal += s.EncodeMs
	}
	warmCount := len(segments) - 1
	out := encodeTimingSummary{
		ColdSegMs:    segments[0].EncodeMs,
		WarmSegCount: warmCount,
		WarmTotalMs:  warmTotal,
	}
	if warmCount > 0 {
		out.WarmAvgSegMs = warmTotal / int64(warmCount)
	}
	if mediaDurationSec > 0 {
		var total int64
		for _, s := range segments {
			total += s.EncodeMs
		}
		if total > 0 {
			rt := mediaDurationSec / (float64(total) / 1000.0)
			out.ThroughputRealtime = &rt
		}
	}
	return out
}

package ops

import "math"

// HLSVODSegmentAction is the host/library decision for an on-demand segment request.
type HLSVODSegmentAction int

const (
	HLSVODActionServe HLSVODSegmentAction = iota
	HLSVODActionWait
	HLSVODActionRestart
)

// HLSVODJobView is a snapshot of the active encode job for segment decisions.
type HLSVODJobView struct {
	Running    bool
	StartIndex int
	// EncodeHead is the segment index ffmpeg is currently filling (smallest not-yet-complete index).
	EncodeHead int
}

// HLSVODDecisionConfig tunes Jellyfin-style wait/restart thresholds.
type HLSVODDecisionConfig struct {
	SegmentSec   float64
	GapMediaSec  float64
	SegmentCount int
}

// HLSVODMaxWaitSegmentGap returns how many segment indices ahead of EncodeHead may wait
// instead of restarting (Jellyfin: 24s of media by default).
func HLSVODMaxWaitSegmentGap(segmentSec, gapMediaSec float64) int {
	if segmentSec <= 0 {
		segmentSec = DefaultHLSSegmentDurationSec
	}
	if gapMediaSec <= 0 {
		gapMediaSec = DefaultHLSVODForwardGapSec
	}
	gap := int(math.Floor(gapMediaSec/segmentSec))
	if gap < 1 {
		gap = 1
	}
	return gap
}

// DecideHLSVODSegmentAction implements the Model A serve/wait/restart rules (Jellyfin DynamicHls-style).
func DecideHLSVODSegmentAction(cfg HLSVODDecisionConfig, job HLSVODJobView, k int, ready bool) HLSVODSegmentAction {
	if ready {
		return HLSVODActionServe
	}
	if cfg.SegmentCount > 0 && k >= cfg.SegmentCount {
		k = cfg.SegmentCount - 1
	}
	if !job.Running {
		return HLSVODActionRestart
	}
	if k < job.StartIndex {
		return HLSVODActionRestart
	}
	gap := HLSVODMaxWaitSegmentGap(cfg.SegmentSec, cfg.GapMediaSec)
	if k-job.EncodeHead <= gap {
		return HLSVODActionWait
	}
	return HLSVODActionRestart
}

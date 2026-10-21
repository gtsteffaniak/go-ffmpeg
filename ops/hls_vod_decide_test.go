package ops

import "testing"

func TestDecideHLSVODSegmentAction(t *testing.T) {
	t.Parallel()
	cfg := HLSVODDecisionConfig{SegmentSec: 4, GapMediaSec: 24, SegmentCount: 100}
	job := HLSVODJobView{Running: true, StartIndex: 10, EncodeHead: 10}

	if DecideHLSVODSegmentAction(cfg, job, 10, true) != HLSVODActionServe {
		t.Fatal("ready should serve")
	}
	if DecideHLSVODSegmentAction(cfg, job, 10, false) != HLSVODActionWait {
		t.Fatal("at head should wait")
	}
	if DecideHLSVODSegmentAction(cfg, job, 12, false) != HLSVODActionWait {
		t.Fatal("within 24s gap should wait")
	}
	if DecideHLSVODSegmentAction(cfg, job, 17, false) != HLSVODActionRestart {
		t.Fatal("far forward should restart")
	}
	if DecideHLSVODSegmentAction(cfg, job, 8, false) != HLSVODActionRestart {
		t.Fatal("backward missing should restart")
	}

	idle := HLSVODJobView{Running: false}
	if DecideHLSVODSegmentAction(cfg, idle, 5, false) != HLSVODActionRestart {
		t.Fatal("no job should restart")
	}

	// After restart at k, head tracks k — must not restart-loop on the same k.
	restarted := HLSVODJobView{Running: true, StartIndex: 40, EncodeHead: 40}
	if DecideHLSVODSegmentAction(cfg, restarted, 40, false) != HLSVODActionWait {
		t.Fatal("post-restart head at k should wait for k, not restart again")
	}
}

func TestDecideSeekBackThenFarForwardRestarts(t *testing.T) {
	t.Parallel()
	cfg := HLSVODDecisionConfig{SegmentSec: 4, GapMediaSec: 24}
	job := HLSVODJobView{Running: true, StartIndex: 10, EncodeHead: 10}
	gap := HLSVODMaxWaitSegmentGap(4, 24)
	k := 10 + gap + 2
	if DecideHLSVODSegmentAction(cfg, job, k, false) != HLSVODActionRestart {
		t.Fatalf("segment %d should restart (not wait) from job head %d", k, job.EncodeHead)
	}
}

func TestHLSVODMaxWaitSegmentGap(t *testing.T) {
	t.Parallel()
	if g := HLSVODMaxWaitSegmentGap(4, 24); g != 6 {
		t.Fatalf("gap=%d want 6", g)
	}
}

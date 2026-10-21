package ops

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func newTestVODJob(startIndex int) *HLSVODSegmentJob {
	return &HLSVODSegmentJob{
		ffmpegDone: make(chan struct{}),
		startIndex: startIndex,
	}
}

func TestInstallJobResetsHeadOnBackwardRestart(t *testing.T) {
	t.Parallel()
	s := NewHLSVODSession(HLSVODSessionOptions{
		OutDir: t.TempDir(),
		Decision: HLSVODDecisionConfig{SegmentSec: 4, GapMediaSec: 24},
	})
	s.head = 50
	s.InstallJob(newTestVODJob(10), false)
	view := s.jobView()
	if view.EncodeHead != 10 {
		t.Fatalf("EncodeHead=%d want 10 after backward restart", view.EncodeHead)
	}
	gap := HLSVODMaxWaitSegmentGap(4, 24)
	k := 10 + gap + 1
	if DecideHLSVODSegmentAction(s.cfg, view, k, false) != HLSVODActionRestart {
		t.Fatalf("far forward after backward restart should restart, not wait")
	}
}

func TestJobViewDeadJobNotRunning(t *testing.T) {
	t.Parallel()
	s := NewHLSVODSession(HLSVODSessionOptions{OutDir: t.TempDir()})
	job := newTestVODJob(2)
	s.InstallJob(job, false)
	close(job.ffmpegDone)
	view := s.jobView()
	if view.Running {
		t.Fatal("exited job should not be running")
	}
	if DecideHLSVODSegmentAction(s.cfg, view, 2, false) != HLSVODActionRestart {
		t.Fatal("dead job should trigger restart, not wait")
	}
}

func TestPerformRestartWaitsForActiveServe(t *testing.T) {
	t.Parallel()
	s := NewHLSVODSession(HLSVODSessionOptions{OutDir: t.TempDir()})
	job := newTestVODJob(0)
	s.InstallJob(job, false)
	s.AcquireServe()
	var started atomic.Int32
	const wantStart = 20
	startJob := func() (*HLSVODSegmentJob, error) {
		started.Add(1)
		return newTestVODJob(wantStart), nil
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		s.ReleaseServe()
		close(job.ffmpegDone)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.performRestart(ctx, wantStart, startJob); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if started.Load() != 1 {
		t.Fatalf("startJob calls=%d want 1", started.Load())
	}
	s.mu.Lock()
	got := s.jobStart
	s.mu.Unlock()
	if got != wantStart {
		t.Fatalf("jobStart=%d want %d", got, wantStart)
	}
}

func TestPerformRestartSingleflight(t *testing.T) {
	t.Parallel()
	s := NewHLSVODSession(HLSVODSessionOptions{OutDir: t.TempDir()})
	var starts int32
	startJob := func() (*HLSVODSegmentJob, error) {
		atomic.AddInt32(&starts, 1)
		return newTestVODJob(10), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.performRestart(ctx, 10, startJob); err != nil {
		t.Fatal(err)
	}
	if err := s.performRestart(ctx, 10, startJob); err != nil {
		t.Fatal(err)
	}
	if starts != 1 {
		t.Fatalf("startJob calls=%d want 1", starts)
	}
	s.mu.Lock()
	j := s.job
	s.mu.Unlock()
	if j != nil {
		close(j.ffmpegDone)
	}
}

func TestExplicitCopySegmentNotReadyUntilNextListed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	segDir := filepath.Join(dir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Minimal publishable fragment (from mp4 tests pattern - use existing test helper if any)
	// Write a file that passes moof check but segment is incomplete logically.
	path := filepath.Join(segDir, "00001.m4s")
	if err := os.WriteFile(path, minimalPublishableFMP4Fragment(), 0o644); err != nil {
		t.Fatal(err)
	}
	ready, err := HLSVODExplicitCopySegmentReady(dir, 1, true)
	if err != nil || ready {
		t.Fatalf("partial explicit segment ready=%v err=%v", ready, err)
	}
	// List segment 2 as complete.
	listPath := hlsVODSegmentListPath(dir)
	if err := os.WriteFile(listPath, []byte("0,0,1,seg/00002.m4s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ready, err = HLSVODExplicitCopySegmentReady(dir, 1, true)
	if err != nil || !ready {
		t.Fatalf("after k+1 listed ready=%v err=%v", ready, err)
	}
}

func minimalPublishableFMP4Fragment() []byte {
	const timescale = 90000
	trunPayload := []byte{0, 0, 0x01, 0x00}
	trunPayload = appendU32(trunPayload, 2)
	trunPayload = appendU32(trunPayload, timescale)
	trunPayload = appendU32(trunPayload, timescale)
	trun := makeAtom("trun", trunPayload)
	tfhd := makeFullAtom("tfhd", 0, append([]byte{0, 0, 0, 0x01}, appendU32(nil, 1)...))
	tfdt := makeFullAtom("tfdt", 0, appendU32(nil, 0))
	trafPayload := append(tfhd, tfdt...)
	trafPayload = append(trafPayload, trun...)
	traf := makeAtom("traf", trafPayload)
	moof := makeAtom("moof", traf)
	mdat := makeAtom("mdat", make([]byte, 9000))
	return append(moof, mdat...)
}

func appendU32(b []byte, v uint32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

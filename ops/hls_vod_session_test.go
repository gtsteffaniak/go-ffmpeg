package ops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestVODJob(startIndex int) *HLSVODSegmentJob {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	j := &HLSVODSegmentJob{
		ffmpegDone: done,
		startIndex: startIndex,
		stop:       cancel,
	}
	go func() {
		<-ctx.Done()
		close(done)
	}()
	return j
}

func TestInstallJobResetsHeadOnBackwardRestart(t *testing.T) {
	t.Parallel()
	s := NewHLSVODSession(HLSVODSessionOptions{
		OutDir:   t.TempDir(),
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

func TestPerformRestartDoesNotWaitForSlowReader(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	segDir := filepath.Join(dir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const wantStart = 20
	segPath := filepath.Join(segDir, "00005.m4s")
	original := minimalPublishableFMP4Fragment()
	if err := os.WriteFile(segPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewHLSVODSession(HLSVODSessionOptions{OutDir: dir})
	old := newTestVODJob(0)
	s.InstallJob(old, false)

	f, err := os.Open(segPath)
	if err != nil {
		t.Fatal(err)
	}
	s.AcquireServePath(segPath)

	var started atomic.Int32
	startJob := func() (*HLSVODSegmentJob, error) {
		started.Add(1)
		return newTestVODJob(wantStart), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	if err := s.performRestart(ctx, wantStart, startJob); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("restart waited %v for slow reader; want immediate cancel", elapsed)
	}
	if started.Load() != 1 {
		t.Fatalf("startJob calls=%d want 1", started.Load())
	}
	if old.Running() {
		t.Fatal("old job still running after restart")
	}
	s.mu.Lock()
	got := s.jobStart
	s.mu.Unlock()
	if got != wantStart {
		t.Fatalf("jobStart=%d want %d", got, wantStart)
	}

	gotBytes, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotBytes) != len(original) {
		t.Fatalf("reader got %d bytes want %d after restart unlink", len(gotBytes), len(original))
	}
	f.Close()
	s.ReleaseServePath(segPath)

	s.mu.Lock()
	j := s.job
	s.mu.Unlock()
	if j != nil {
		j.Cancel()
		_ = j.Wait()
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

func TestPerformRestartDoesNotUnlinkReadyCachedSegment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const stopIndex = 7
	readyPath := writeReadyVODSegment(t, dir, stopIndex)

	s := NewHLSVODSession(HLSVODSessionOptions{OutDir: dir, ExplicitCopyCuts: true})
	job := newTestVODJob(2)
	job.stopIndex = stopIndex
	s.InstallJob(job, true)
	s.mu.Lock()
	s.head = stopIndex
	s.partialPath = s.partialSegmentPathLocked(stopIndex, job)
	s.mu.Unlock()

	startJob := func() (*HLSVODSegmentJob, error) {
		return newTestVODJob(2), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.performRestart(ctx, stopIndex, startJob); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(readyPath); err != nil {
		t.Fatalf("ready cached segment must survive restart unlink: %v", err)
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

func writeReadyVODSegment(t *testing.T, dir string, index int) string {
	t.Helper()
	segDir := filepath.Join(dir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(segDir, fmt.Sprintf("%05d.m4s", index))
	if err := os.WriteFile(path, minimalPublishableFMP4Fragment(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRefreshEncodeHeadDoesNotSpinWhenJobCleared(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const idx = 5
	writeReadyVODSegment(t, dir, idx)

	s := NewHLSVODSession(HLSVODSessionOptions{OutDir: dir})
	job := newTestVODJob(idx)
	s.InstallJob(job, false)

	s.mu.Lock()
	s.job = nil
	s.mu.Unlock()

	start := time.Now()
	s.refreshEncodeHead()
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("refreshEncodeHead took %v; want prompt return when job cleared", elapsed)
	}
}

func TestWaitForSegmentReturnsWhenRestartStartJobFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewHLSVODSession(HLSVODSessionOptions{
		OutDir:       dir,
		Decision:     HLSVODDecisionConfig{SegmentSec: 4, GapMediaSec: 24},
		PollInterval: 5 * time.Millisecond,
	})
	job := newTestVODJob(0)
	s.InstallJob(job, false)
	job.stop()
	<-job.ffmpegDone

	wantErr := errors.New("start failed")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err := s.WaitForSegment(ctx, 10, func() (*HLSVODSegmentJob, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WaitForSegment err=%v want %v", err, wantErr)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("WaitForSegment took %v after startJob failure; want prompt error", elapsed)
	}
}

func TestHLSVODSessionConcurrentRefreshWaitInstallRace(t *testing.T) {
	dir := t.TempDir()
	writeReadyVODSegment(t, dir, 0)
	writeReadyVODSegment(t, dir, 1)

	s := NewHLSVODSession(HLSVODSessionOptions{
		OutDir:       dir,
		Decision:     HLSVODDecisionConfig{SegmentSec: 4, GapMediaSec: 24},
		PollInterval: 2 * time.Millisecond,
	})

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	stop := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				switch id % 4 {
				case 0:
					s.refreshEncodeHead()
				case 1:
					_ = s.jobView()
				case 2:
					s.InstallJob(newTestVODJob(id%3), id%2 == 0)
				case 3:
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
					f, err := s.WaitForSegment(ctx, 0, func() (*HLSVODSegmentJob, error) {
						return newTestVODJob(0), nil
					})
					if err == nil {
						_ = f.Close()
						s.ReleaseServePath(HLSVODSegmentPath(dir, 0))
					}
					cancel()
				}
			}
		}(i)
	}
	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()

	s.mu.Lock()
	j := s.job
	s.mu.Unlock()
	if j != nil {
		j.Cancel()
		_ = j.Wait()
	}
}

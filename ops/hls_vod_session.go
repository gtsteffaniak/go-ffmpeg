package ops

import (
	"context"
	"os"
	"sync"
	"time"
)

// HLSVODSession coordinates one cache directory / transcode profile (Jellyfin DynamicHls-style).
type HLSVODSession struct {
	mu        sync.Mutex
	restartMu sync.Mutex

	outDir string
	cfg    HLSVODDecisionConfig

	job          *HLSVODSegmentJob
	jobStart     int
	head         int
	explicitCopy bool

	activeServes int
	partialPath  string

	pollInterval time.Duration
}

// HLSVODSessionOptions configures wait polling for WaitForSegment.
type HLSVODSessionOptions struct {
	OutDir       string
	Decision     HLSVODDecisionConfig
	PollInterval time.Duration
	// ExplicitCopyCuts enables -f segment completion checks (live CSV list / k+1).
	ExplicitCopyCuts bool
}

// NewHLSVODSession returns a session for outDir.
func NewHLSVODSession(opts HLSVODSessionOptions) *HLSVODSession {
	poll := opts.PollInterval
	if poll <= 0 {
		poll = 100 * time.Millisecond
	}
	return &HLSVODSession{
		outDir:         opts.OutDir,
		cfg:            opts.Decision,
		pollInterval:   poll,
		explicitCopy:   opts.ExplicitCopyCuts,
	}
}

// AcquireServe increments the active serve refcount (protects open files from deletion).
func (s *HLSVODSession) AcquireServe() {
	s.mu.Lock()
	s.activeServes++
	s.mu.Unlock()
}

// ReleaseServe decrements the active serve refcount.
func (s *HLSVODSession) ReleaseServe() {
	s.mu.Lock()
	s.activeServes--
	if s.activeServes < 0 {
		s.activeServes = 0
	}
	s.mu.Unlock()
}

// SetJob records a newly started encode job.
func (s *HLSVODSession) SetJob(job *HLSVODSegmentJob) {
	s.InstallJob(job, s.explicitCopy)
}

// InstallJob registers a running ffmpeg job and starts progress tracking.
func (s *HLSVODSession) InstallJob(job *HLSVODSegmentJob, explicitCopyCuts bool) {
	s.attachJob(job, explicitCopyCuts)
}

func (s *HLSVODSession) attachJob(job *HLSVODSegmentJob, explicitCopy bool) {
	if job == nil {
		return
	}
	s.mu.Lock()
	s.job = job
	s.jobStart = job.StartIndex()
	s.head = job.StartIndex()
	s.explicitCopy = explicitCopy
	s.partialPath = HLSVODSegmentPath(s.outDir, s.head)
	s.mu.Unlock()
	go s.watchJob(job)
}

func (s *HLSVODSession) watchJob(job *HLSVODSegmentJob) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		if !job.Running() {
			s.clearJobIf(job)
			return
		}
		s.refreshEncodeHead()
		s.mu.Lock()
		if s.job == job {
			s.partialPath = HLSVODSegmentPath(s.outDir, s.head)
		}
		s.mu.Unlock()
		select {
		case <-job.ffmpegDone:
			s.clearJobIf(job)
			return
		case <-ticker.C:
		}
	}
}

func (s *HLSVODSession) clearJobIf(job *HLSVODSegmentJob) {
	s.mu.Lock()
	if s.job == job {
		s.job = nil
	}
	s.mu.Unlock()
	s.refreshEncodeHead()
}

func (s *HLSVODSession) refreshEncodeHead() {
	s.mu.Lock()
	job := s.job
	jobStart := s.jobStart
	explicit := s.explicitCopy
	s.mu.Unlock()
	if job == nil {
		return
	}
	if explicit {
		head, err := hlsVODSegmentListHead(s.outDir, jobStart)
		if err == nil {
			s.mu.Lock()
			if s.job == job {
				s.head = head
			}
			s.mu.Unlock()
		}
		return
	}
	// HLS temp_file path: advance head while consecutive segments are publishable.
	for {
		path := HLSVODSegmentPath(s.outDir, s.head)
		ready, err := hlsVODSegmentReadyAt(path)
		if err != nil || !ready {
			break
		}
		s.mu.Lock()
		if s.job == job {
			s.head++
		}
		s.mu.Unlock()
	}
}

func (s *HLSVODSession) jobView() HLSVODJobView {
	s.refreshEncodeHead()
	s.mu.Lock()
	defer s.mu.Unlock()
	head := s.head
	if s.job == nil {
		return HLSVODJobView{Running: false, EncodeHead: head}
	}
	if !s.job.Running() {
		s.job = nil
		return HLSVODJobView{Running: false, EncodeHead: head}
	}
	return HLSVODJobView{
		Running:    true,
		StartIndex: s.jobStart,
		EncodeHead: head,
	}
}

func (s *HLSVODSession) jobRunning() bool {
	s.mu.Lock()
	job := s.job
	s.mu.Unlock()
	return job != nil && job.Running()
}

// WaitForSegment blocks until segment k is ready, the context is canceled, or an error occurs.
func (s *HLSVODSession) WaitForSegment(ctx context.Context, k int, startJob func() (*HLSVODSegmentJob, error)) (*os.File, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var f *os.File
		var ready bool
		var err error
		if s.explicitCopy {
			f, ready, err = HLSVODOpenReadyExplicitCopySegment(s.outDir, k, s.jobRunning())
		} else {
			path := HLSVODSegmentPath(s.outDir, k)
			f, ready, err = HLSVODOpenReadySegment(path)
		}
		if err != nil {
			return nil, err
		}
		if ready {
			s.refreshEncodeHead()
			s.AcquireServe()
			return f, nil
		}
		if f != nil {
			f.Close()
		}

		view := s.jobView()
		action := DecideHLSVODSegmentAction(s.cfg, view, k, false)
		switch action {
		case HLSVODActionWait:
			if err := sleepCtx(ctx, s.pollInterval); err != nil {
				return nil, err
			}
			continue
		case HLSVODActionRestart:
			if startJob == nil {
				return nil, context.Canceled
			}
			if err := s.performRestart(ctx, k, startJob); err != nil {
				return nil, err
			}
			continue
		default:
			if err := sleepCtx(ctx, s.pollInterval); err != nil {
				return nil, err
			}
		}
	}
}

func (s *HLSVODSession) performRestart(ctx context.Context, k int, startJob func() (*HLSVODSegmentJob, error)) error {
	s.restartMu.Lock()
	defer s.restartMu.Unlock()

	if DecideHLSVODSegmentAction(s.cfg, s.jobView(), k, false) != HLSVODActionRestart {
		return nil
	}

	for {
		s.mu.Lock()
		serves := s.activeServes
		s.mu.Unlock()
		if serves == 0 {
			break
		}
		if err := sleepCtx(ctx, s.pollInterval); err != nil {
			return err
		}
	}

	s.mu.Lock()
	old := s.job
	s.job = nil
	partial := s.partialPath
	s.mu.Unlock()

	if old != nil {
		old.Cancel()
		_ = old.Wait()
	}
	s.removePartialAt(partial)

	job, err := startJob()
	if err != nil {
		return err
	}
	if job != nil {
		s.mu.Lock()
		needInstall := s.job != job
		explicit := s.explicitCopy
		s.mu.Unlock()
		if needInstall {
			s.InstallJob(job, explicit)
		}
	}
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (s *HLSVODSession) removePartialAt(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
	_ = os.Remove(path + hlsVODSegmentTmpSuffix)
}

// TrackPartialSegment marks the path ffmpeg is currently writing (for cleanup on kill).
func (s *HLSVODSession) TrackPartialSegment(path string) {
	s.mu.Lock()
	s.partialPath = path
	s.mu.Unlock()
}

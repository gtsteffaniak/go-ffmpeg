package ops

import (
	"context"
	"os"
	"sync"
	"time"
)

// HLSVODSession coordinates one cache directory / transcode profile (Jellyfin DynamicHls-style).
type HLSVODSession struct {
	mu sync.Mutex

	outDir string
	cfg    HLSVODDecisionConfig

	job      *HLSVODSegmentJob
	jobStart int
	head     int

	activeServes int
	partialPath  string

	pollInterval time.Duration
}

// HLSVODSessionOptions configures wait polling for WaitForSegment.
type HLSVODSessionOptions struct {
	OutDir       string
	Decision     HLSVODDecisionConfig
	PollInterval time.Duration
}

// NewHLSVODSession returns a session for outDir.
func NewHLSVODSession(opts HLSVODSessionOptions) *HLSVODSession {
	poll := opts.PollInterval
	if poll <= 0 {
		poll = 100 * time.Millisecond
	}
	return &HLSVODSession{
		outDir:       opts.OutDir,
		cfg:          opts.Decision,
		pollInterval: poll,
	}
}

// NoteSegmentReady updates encode head after segment index is observed ready on disk.
func (s *HLSVODSession) NoteSegmentReady(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index >= s.head {
		s.head = index + 1
	}
}

// AcquireServe increments the active serve refcount for the current job output.
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

// SetJob records a newly started encode job (caller must not hold s.mu).
func (s *HLSVODSession) SetJob(job *HLSVODSegmentJob) {
	if job == nil {
		return
	}
	s.mu.Lock()
	s.job = job
	s.jobStart = job.StartIndex()
	if s.head < s.jobStart {
		s.head = s.jobStart
	}
	s.mu.Unlock()
}

// jobView snapshots job state under lock.
func (s *HLSVODSession) jobView() (HLSVODJobView, *HLSVODSegmentJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job == nil {
		return HLSVODJobView{Running: false, StartIndex: 0, EncodeHead: s.head}, nil
	}
	return HLSVODJobView{
		Running:    true,
		StartIndex: s.jobStart,
		EncodeHead: s.head,
	}, s.job
}

// WaitForSegment blocks until segment k is ready, the context is canceled, or an error occurs.
// startJob is called outside the session lock when a restart is required.
func (s *HLSVODSession) WaitForSegment(ctx context.Context, k int, startJob func() (*HLSVODSegmentJob, error)) (*os.File, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path := HLSVODSegmentPath(s.outDir, k)
		f, ready, err := HLSVODOpenReadySegment(path)
		if err != nil {
			return nil, err
		}
		if ready {
			s.NoteSegmentReady(k)
			s.AcquireServe()
			return f, nil
		}
		if f != nil {
			f.Close()
		}

		view, _ := s.jobView()
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
			s.cancelJobIfIdle()
			job, err := startJob()
			if err != nil {
				return nil, err
			}
			s.SetJob(job)
			if err := sleepCtx(ctx, s.pollInterval); err != nil {
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

// cancelJobIfIdle stops the running job when no callers are serving its output.
func (s *HLSVODSession) cancelJobIfIdle() {
	s.mu.Lock()
	if s.job == nil || s.activeServes > 0 {
		s.mu.Unlock()
		return
	}
	job := s.job
	s.job = nil
	s.mu.Unlock()
	s.removePartialTail()
	if job != nil {
		job.Cancel()
	}
}

// removePartialTail deletes only the in-progress partial segment (temp sibling), not completed fragments.
func (s *HLSVODSession) removePartialTail() {
	s.mu.Lock()
	partial := s.partialPath
	s.partialPath = ""
	s.mu.Unlock()
	if partial != "" {
		_ = os.Remove(partial)
		_ = os.Remove(partial + hlsVODSegmentTmpSuffix)
	}
}

// TrackPartialSegment marks the path ffmpeg is currently writing (for cleanup on kill).
func (s *HLSVODSession) TrackPartialSegment(path string) {
	s.mu.Lock()
	s.partialPath = path
	s.mu.Unlock()
}

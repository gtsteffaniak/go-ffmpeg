package ops

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

// HLSContinuousOptions configures a long-running ffmpeg -f hls job writing fMP4 segments to disk.
type HLSContinuousOptions struct {
	Input      InputSource
	OutputDir  string // cache job directory (init.m4s + seg/%05d.m4s)
	StartIndex int
	StartSec   float64
	// SegmentDurations[i] is the #EXTINF duration for segment index i on the HLS media
	// timeline. When set, fMP4 fragments are aligned to a contiguous decode timeline
	// (sum of prior durations) instead of source keyframe timestamps.
	SegmentDurations []float64
	// SegmentMediaStarts is deprecated for timeline alignment; kept for diagnostics only.
	SegmentMediaStarts []float64
	SegmentSec         float64
	FreshPlaylist      bool // use temp_file instead of append_list at StartIndex 0
	Decode             encode.VideoDecodeProfile
	Profile            encode.VideoProfile
	MaxHeight          int
	Remux              bool
	VideoCopy          bool
	GOP                int
	// Pacing selects cache-fill vs live-paced input when Throttle is nil.
	Pacing HLSContinuousPacing
	// Throttle, when non-nil, overrides Pacing — including Enabled: false.
	Throttle *encode.ThrottleConfig
}

// HLSContinuousJob runs ffmpeg until EOF, cancellation, or error.
type HLSContinuousJob struct {
	ffmpegDone chan struct{}
	alignDone  chan struct{}
	stop       context.CancelFunc
	stderrMon  *vtDecodeStderrMonitor
	mu         sync.Mutex
	err        error
}

// VTDecodeUnreliable reports sustained VideoToolbox hardware decode failures in stderr.
func (j *HLSContinuousJob) VTDecodeUnreliable() bool {
	if j == nil {
		return false
	}
	return j.stderrMon.VTDecodeUnreliable()
}

// Cancel stops the ffmpeg process.
func (j *HLSContinuousJob) Cancel() {
	if j == nil {
		return
	}
	if j.stop != nil {
		j.stop()
	}
}

// Wait blocks until the job finishes and segment timeline alignment completes.
func (j *HLSContinuousJob) Wait() error {
	if j == nil {
		return nil
	}
	if j.alignDone != nil {
		<-j.alignDone
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.err
}

// Done returns a channel that receives when ffmpeg exits (before timeline alignment).
func (j *HLSContinuousJob) Done() <-chan error {
	if j == nil {
		ch := make(chan error, 1)
		ch <- nil
		close(ch)
		return ch
	}
	ch := make(chan error, 1)
	go func() {
		if j.ffmpegDone != nil {
			<-j.ffmpegDone
		}
		j.mu.Lock()
		err := j.err
		j.mu.Unlock()
		ch <- err
		close(ch)
	}()
	return ch
}

// StartHLSContinuous launches ffmpeg -f hls writing segments under OutputDir.
func StartHLSContinuous(parent context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSContinuousOptions) (*HLSContinuousJob, error) {
	if runner == nil || caps == nil {
		return nil, fmt.Errorf("hls continuous: runner or capabilities missing")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return nil, fmt.Errorf("hls continuous: output dir required")
	}
	if opts.SegmentSec <= 0 {
		opts.SegmentSec = DefaultHLSSegmentDurationSec
	}
	outDir, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("hls continuous: output dir: %w", err)
	}
	opts.OutputDir = outDir
	inputURL := opts.Input.URL
	if inputURL != "" {
		if abs, absErr := filepath.Abs(inputURL); absErr == nil {
			inputURL = abs
		}
		opts.Input = InputSource{URL: inputURL, StreamType: opts.Input.StreamType}
	}
	if err := os.MkdirAll(filepath.Join(outDir, "seg"), 0o755); err != nil {
		return nil, err
	}

	args, err := buildHLSContinuousArgs(runner, caps, opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(parent)
	cmd := ffexec.CommandContext(ctx, runner.FFmpegPath, args...)
	// HLS segment paths with subdirs (seg/%05d.m4s) resolve from the process working
	// directory, not the playlist path — run ffmpeg inside the cache job directory.
	cmd.Dir = outDir
	stderr := newStderrTail(continuousStderrTailMax)
	var dst io.Writer = stderr
	if runner.VerboseFFmpeg {
		dst = io.MultiWriter(os.Stderr, stderr)
	}
	stderrMon := newVTDecodeStderrMonitor(dst)
	cmd.Stderr = stderrMon

	job := &HLSContinuousJob{
		ffmpegDone: make(chan struct{}),
		alignDone:  make(chan struct{}),
		stop:       cancel,
		stderrMon:  stderrMon,
	}
	go func() {
		err := cmd.Run()
		if ctx.Err() != nil {
			err = ctx.Err()
		} else if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg != "" {
				err = fmt.Errorf("%w: %s", err, msg)
			}
		}
		job.mu.Lock()
		job.err = err
		job.mu.Unlock()
		close(job.ffmpegDone)
	}()
	go runContinuousSegmentAligner(ctx, outDir, opts, job)
	return job, nil
}

func buildHLSContinuousArgs(runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSContinuousOptions) ([]string, error) {
	segDur := opts.SegmentSec
	gop := opts.GOP
	if gop <= 0 {
		gop = HLSSegmentGOP(defaultHLSSegmentFPS, DefaultOnDemandHLSDefaults())
	}

	args := []string{"-hide_banner", "-nostats", "-loglevel", hlsFFmpegLogLevel(runner), "-y"}
	videoCopyPipeline := opts.Remux || opts.VideoCopy
	if opts.StartSec > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", opts.StartSec))
	}
	if videoCopyPipeline {
		args = appendHLSInputTimestampFlags(args)
	}
	throttle := resolveContinuousThrottle(opts)
	var inputExtra *InputExtraFlags
	if throttle.Enabled {
		inputExtra = &InputExtraFlags{Throttle: &throttle, Features: caps.FeatureFlags}
	}
	args = appendInputFlags(args, opts.Input, nil, inputExtra)
	if !videoCopyPipeline {
		resolver := encode.NewResolver(caps)
		if initArgs := resolver.VideoToolboxPreInputInit(opts.Decode, opts.Profile); len(initArgs) > 0 {
			args = append(args, initArgs...)
		}
		decodeArgs, err := resolver.VideoDecoderArgs(opts.Decode)
		if err != nil {
			return nil, err
		}
		args = append(args, decodeArgs...)
		// Tolerate occasional EAC3/MKV demux glitches during long transcode runs.
		args = append(args, "-fflags", "+discardcorrupt+genpts")
	}
	args = append(args, "-i", opts.Input.URL)
	args = append(args, "-map", "0:v:0")
	if !opts.VideoOnly() {
		args = append(args, "-map", "0:a:0?")
	}
	args = append(args, "-sn", "-dn")

	switch {
	case opts.Remux:
		args = append(args, "-c:v", "copy")
		if !opts.VideoOnly() {
			args = append(args, "-c:a", "copy")
		}
	case opts.VideoCopy:
		args = append(args, "-c:v", "copy")
		if !opts.VideoOnly() {
			args = append(args,
				"-c:a", "aac",
				"-ar", "48000",
				"-ac", "2",
				"-profile:a", "aac_low",
				"-af", "aresample=async=1:first_pts=0",
				"-bsf:a", "aac_adtstoasc",
			)
		}
	default:
		resolver := encode.NewResolver(caps)
		vidArgs, err := resolver.VideoEncoderArgs(opts.Profile)
		if err != nil {
			return nil, err
		}
		filterArgs, err := resolver.VideoFilterArgs(opts.Profile, opts.Decode, opts.MaxHeight)
		if err != nil {
			return nil, err
		}
		args = append(args, filterArgs...)
		args = append(args, vidArgs...)
		encSel, encErr := resolver.ResolveEncoder(opts.Profile)
		if encErr == nil && (opts.Profile.Codec == "" || opts.Profile.Codec == encode.CodecH264) {
			args = encode.AppendH264FMP4CompatArgs(args, encSel.Accel, opts.MaxHeight)
		}
		x264Only := encErr == nil && (encSel.Accel == capabilities.AccelNone || encSel.Accel == "")
		args = append(args, "-video_track_timescale", "90000", "-g", fmt.Sprintf("%d", gop), "-keyint_min", fmt.Sprintf("%d", gop))
		if x264Only {
			args = append(args, "-sc_threshold", "0")
		}
		args = append(args, "-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%.0f)", segDur))
		if !opts.VideoOnly() {
			args = append(args,
				"-c:a", "aac",
				"-ar", "48000",
				"-ac", "2",
				"-profile:a", "aac_low",
				"-af", "aresample=async=1:first_pts=0",
			)
		}
	}

	if videoCopyPipeline {
		offsetSec := 0.0
		if opts.StartIndex > 0 || opts.StartSec > 0 {
			offsetSec = opts.StartSec
		}
		args = appendHLSOutputTimestampArgs(args, offsetSec)
	} else {
		args = appendHLSTranscodeCopyTimestampArgs(args)
	}
	hlsFlags := "append_list"
	if opts.StartIndex > 0 || opts.FreshPlaylist {
		// Mid-file restarts and post-invalidation jobs must not append to a stale playlist.
		hlsFlags = "temp_file"
	}
	args = append(args,
		"-max_muxing_queue_size", "128",
		"-f", "hls",
		"-max_delay", "5000000",
		"-hls_time", fmt.Sprintf("%.3f", segDur),
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "init.m4s",
		"-hls_segment_filename", "seg/%05d.m4s",
		"-start_number", fmt.Sprintf("%d", opts.StartIndex),
		"-hls_playlist_type", "vod",
		"-hls_list_size", "0",
		"-hls_flags", hlsFlags,
	)
	if !videoCopyPipeline {
		args = append(args, "-hls_segment_options", "movflags=+frag_discont")
	}
	args = append(args, "ffmpeg.m3u8")
	return args, nil
}

func (o HLSContinuousOptions) VideoOnly() bool {
	return false
}

const continuousStderrTailMax = 64 << 10

// stderrTail keeps a bounded tail of ffmpeg stderr so long-running continuous
// jobs cannot grow an unbounded capture buffer.
type stderrTail struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func newStderrTail(max int) *stderrTail {
	if max <= 0 {
		max = continuousStderrTailMax
	}
	return &stderrTail{max: max}
}

func (s *stderrTail) Write(p []byte) (int, error) {
	if s == nil {
		return len(p), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(s.buf, p...)
	if len(s.buf) > s.max {
		keep := s.buf[len(s.buf)-s.max:]
		s.buf = append([]byte(nil), keep...)
	}
	return len(p), nil
}

func (s *stderrTail) String() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.buf)
}

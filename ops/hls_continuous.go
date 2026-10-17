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
	// StartSec is the seek position on the 0-based HLS media timeline.
	StartSec float64
	// InputStartTimeSec is the container start_time from ffprobe (added for ffmpeg input -ss).
	InputStartTimeSec float64
	SegmentSec        float64
	FreshPlaylist     bool // use temp_file instead of append_list at StartIndex 0
	Decode            encode.VideoDecodeProfile
	Profile           encode.VideoProfile
	MaxHeight         int
	Remux             bool
	VideoCopy         bool
	GOP               int
	// InputVideoCodec names the source video codec (e.g. "hevc"). On stream-copy
	// pipelines it selects the fMP4 sample-entry tag browsers require — ffmpeg
	// defaults HEVC to hev1, which MSE implementations refuse to decode.
	InputVideoCodec string
	// Pacing selects cache-fill vs live-paced input when Throttle is nil.
	Pacing HLSContinuousPacing
	// Throttle, when non-nil, overrides Pacing — including Enabled: false.
	Throttle *encode.ThrottleConfig
	// OmitAudio drops audio streams from the output (VideoOnly() reports this flag).
	OmitAudio bool
}

// VideoOnly reports whether audio is omitted from the continuous HLS output.
func (o HLSContinuousOptions) VideoOnly() bool {
	return o.OmitAudio
}

// HLSContinuousJob runs ffmpeg until EOF, cancellation, or error.
type HLSContinuousJob struct {
	ffmpegDone chan struct{}
	alignDone  chan struct{}
	stop       context.CancelFunc
	stderrMon  *vtDecodeStderrMonitor
	startSec   float64
	mu         sync.Mutex
	err        error
}

// StartSec returns the generation's effective media start in seconds — the
// requested seek position for transcode jobs, or the keyframe the muxer
// actually snapped to for stream-copy jobs. Published fMP4 segments are
// aligned on the full-file decode timeline (tfdt ≈ StartSec + cumulative
// #EXTINF), so players can use native currentTime without subtracting an offset.
func (j *HLSContinuousJob) StartSec() float64 {
	if j == nil {
		return 0
	}
	return j.startSec
}

// VTDecodeUnreliable reports sustained VideoToolbox hardware decode failures in stderr.
func (j *HLSContinuousJob) VTDecodeUnreliable() bool {
	if j == nil {
		return false
	}
	return j.stderrMon.VTDecodeUnreliable()
}

// SoftwareDecodeUnreliable reports sustained software decoder corruption in stderr
// (for example Invalid NAL under readrate pacing). Output segments may buffer but not render.
func (j *HLSContinuousJob) SoftwareDecodeUnreliable() bool {
	if j == nil {
		return false
	}
	return j.stderrMon.SoftwareDecodeUnreliable()
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

// prepareContinuousOutputLayout creates raw/ and seg/ and clears stale publish
// artifacts when starting a new generation (mid-file seek or fresh playlist).
// append_list resume at StartIndex 0 intentionally keeps prior seg/ and playlists.
func prepareContinuousOutputLayout(outDir string, opts HLSContinuousOptions) error {
	// raw/ holds the muxer's unaligned output; the publisher renames validated
	// fragments into seg/, so a file under seg/ is always safe to serve.
	if opts.StartIndex > 0 || opts.FreshPlaylist {
		for _, sub := range []string{"seg", "raw"} {
			if err := os.RemoveAll(filepath.Join(outDir, sub)); err != nil {
				return err
			}
		}
		for _, name := range []string{"ffmpeg.m3u8", continuousServePlaylistName, continuousJobStatusName} {
			if err := os.Remove(filepath.Join(outDir, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	for _, sub := range []string{"seg", "raw"} {
		if err := os.MkdirAll(filepath.Join(outDir, sub), 0o755); err != nil {
			return err
		}
	}
	return nil
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
	if err := prepareContinuousOutputLayout(outDir, opts); err != nil {
		return nil, err
	}

	opts, err = snapContinuousCopyStart(parent, runner, opts)
	if err != nil {
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
		startSec:   opts.StartSec,
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
	go runContinuousSegmentAligner(ctx, outDir, job)
	return job, nil
}

func snapContinuousCopyStart(ctx context.Context, runner *ffexec.Runner, opts HLSContinuousOptions) (HLSContinuousOptions, error) {
	if !opts.Remux && !opts.VideoCopy || opts.StartSec <= hlsStartTimeEpsilonSec {
		return opts, nil
	}
	if runner == nil || strings.TrimSpace(opts.Input.URL) == "" {
		return opts, nil
	}
	keyframes, err := ProbeVideoKeyframeTimes(ctx, runner, opts.Input.URL)
	if err != nil {
		return opts, fmt.Errorf("hls continuous: keyframe snap: %w", err)
	}
	norm := NormalizeHLSKeyframeTimes(keyframes, opts.InputStartTimeSec)
	opts.StartSec = KeyframeSeekBefore(norm, opts.StartSec)
	return opts, nil
}

func buildHLSContinuousArgs(runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSContinuousOptions) ([]string, error) {
	segDur := opts.SegmentSec
	gop := opts.GOP
	if gop <= 0 {
		gop = HLSSegmentGOP(defaultHLSSegmentFPS, DefaultOnDemandHLSDefaults())
	}

	args := []string{"-hide_banner", "-nostats", "-loglevel", hlsFFmpegLogLevel(runner), "-y"}
	videoCopyPipeline := opts.Remux || opts.VideoCopy
	inputSeek := HLSInputSeekSec(opts.StartSec, opts.InputStartTimeSec)
	if inputSeek > hlsStartTimeEpsilonSec {
		args = append(args, "-ss", fmt.Sprintf("%.3f", inputSeek))
	}
	if videoCopyPipeline {
		args = appendHLSInputTimestampFlags(args)
	}
	throttle := resolveContinuousThrottle(opts)
	var inputExtra *InputExtraFlags
	if throttle.Enabled {
		inputExtra = &InputExtraFlags{Throttle: &throttle, Features: caps.FeatureFlags}
	}
	args = appendInputFlags(args, runner, opts.Input, nil, inputExtra)
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
			// Same rule as the transcode path: a seek job must not force
			// first_pts=0 — async resample would pad silence for the offset
			// between the seek point and the first audio packet.
			audioFilter := "aresample=async=1"
			if opts.StartSec <= 0 {
				audioFilter = "aresample=async=1:first_pts=0"
			}
			args = append(args,
				"-c:a", "aac",
				"-ar", "48000",
				"-ac", "2",
				"-profile:a", "aac_low",
				"-af", audioFilter,
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
			// Only force the audio to the media origin when the job starts at 0.
			// A job that seeks keeps source timestamps (-copyts), so forcing
			// first_pts=0 would open an A/V gap as long as the seek offset: ffmpeg
			// rewinds the audio to 0 regardless of the value given, and the HLS
			// muxer then has to write the whole gap as audio-only segments before
			// any video arrives.
			audioFilter := "aresample=async=1"
			if opts.StartSec <= 0 {
				audioFilter = "aresample=async=1:first_pts=0"
			}
			args = append(args,
				"-c:a", "aac",
				"-ar", "48000",
				"-ac", "2",
				"-profile:a", "aac_low",
				"-af", audioFilter,
			)
		}
	}

	if videoCopyPipeline {
		if tag := fmp4CopyVideoTag(opts.InputVideoCodec); tag != "" {
			args = append(args, "-tag:v", tag)
		}
		offsetSec := 0.0
		if opts.StartIndex > 0 || opts.StartSec > 0 {
			offsetSec = opts.StartSec
		}
		args = appendHLSOutputTimestampArgs(args, offsetSec)
	} else {
		args = appendHLSTranscodeCopyTimestampArgs(args)
	}
	// Every fragment begins on a keyframe — copy pipelines split at GOP
	// boundaries and transcode forces keyframes on the segment grid — so the
	// playlist can advertise random access for hls.js seeking.
	hlsFlags := "append_list+independent_segments"
	if opts.StartIndex > 0 || opts.FreshPlaylist {
		// Mid-file restarts and post-invalidation jobs must not append to a stale playlist.
		hlsFlags = "temp_file+independent_segments"
	}
	args = append(args,
		"-max_muxing_queue_size", "128",
		"-f", "hls",
		"-max_delay", "5000000",
		"-hls_time", fmt.Sprintf("%.3f", segDur),
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "init.m4s",
		"-hls_segment_filename", "raw/%05d.m4s",
		"-start_number", fmt.Sprintf("%d", opts.StartIndex),
		// event, not vod: ffmpeg only writes a vod playlist when the stream ends,
		// so a continuous job over a long input published no playlist at all until
		// the transcode finished. An event playlist is written as each segment
		// completes and gains #EXT-X-ENDLIST at EOF.
		"-hls_playlist_type", "event",
		"-hls_list_size", "0",
		"-hls_flags", hlsFlags,
	)
	if !videoCopyPipeline {
		args = append(args, "-hls_segment_options", "movflags=+frag_discont")
	}
	args = append(args, "ffmpeg.m3u8")
	return args, nil
}

// fmp4CopyVideoTag returns the fMP4 sample-entry tag required when
// stream-copying the codec into HLS fMP4. HEVC must be tagged hvc1 — ffmpeg
// writes hev1 by default and MSE decoders (Safari strictly, most others in
// practice) reject it. h264 and VP9 already get browser-compatible tags
// (avc1/vp09) from the muxer.
func fmp4CopyVideoTag(codec string) string {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "hevc", "h265", "hev1", "hvc1":
		return "hvc1"
	}
	return ""
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

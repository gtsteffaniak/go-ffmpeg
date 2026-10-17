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

// HLSVODJobOptions configures a forward-encoding ffmpeg -f hls job for on-demand VOD (Model A).
// StartIndex and StartSec describe where this job continues on the synthetic playlist timeline.
type HLSVODJobOptions struct {
	Input      InputSource
	OutputDir  string
	StartIndex int
	// StartSec is the playlist-local media time of segment StartIndex (starts[i] from VODSegmentList).
	StartSec float64
	// SegmentSec is the transcode grid length (-hls_time and forced keyframe interval).
	SegmentSec float64
	// InputStartTimeSec is the container start_time from ffprobe (added for ffmpeg input -ss).
	InputStartTimeSec float64
	// KeyframeSeekTimes, when set, snap input -ss for remux/video-copy jobs.
	KeyframeSeekTimes []float64
	Decode            encode.VideoDecodeProfile
	Profile           encode.VideoProfile
	MaxHeight         int
	Remux             bool
	VideoCopy         bool
	GOP               int
	InputVideoCodec   string
	OmitAudio         bool
	// ClearSegmentsFromIndex removes seg/%05d.m4s for indices >= StartIndex before starting.
	ClearSegmentsFromIndex bool
}

// VideoOnly reports whether audio is omitted.
func (o HLSVODJobOptions) VideoOnly() bool {
	return o.OmitAudio
}

// HLSVODSegmentJob runs ffmpeg until EOF, cancellation, or error. Segments are published atomically
// via ffmpeg's temp_file HLS flag directly under seg/.
type HLSVODSegmentJob struct {
	ffmpegDone chan struct{}
	stop       context.CancelFunc
	stderrMon  *vtDecodeStderrMonitor
	startSec   float64
	startIndex int
	mu         sync.Mutex
	err        error
}

// StartSec returns the playlist timeline anchor for this job (segment StartIndex start time).
func (j *HLSVODSegmentJob) StartSec() float64 {
	if j == nil {
		return 0
	}
	return j.startSec
}

// StartIndex returns the first segment number this job emits.
func (j *HLSVODSegmentJob) StartIndex() int {
	if j == nil {
		return 0
	}
	return j.startIndex
}

// Cancel stops ffmpeg.
func (j *HLSVODSegmentJob) Cancel() {
	if j == nil || j.stop == nil {
		return
	}
	j.stop()
}

// Wait blocks until ffmpeg exits.
func (j *HLSVODSegmentJob) Wait() error {
	if j == nil {
		return nil
	}
	if j.ffmpegDone != nil {
		<-j.ffmpegDone
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.err
}

// Done returns a channel that receives when ffmpeg exits.
func (j *HLSVODSegmentJob) Done() <-chan error {
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

// VTDecodeUnreliable reports sustained VideoToolbox hardware decode failures in stderr.
func (j *HLSVODSegmentJob) VTDecodeUnreliable() bool {
	if j == nil {
		return false
	}
	return j.stderrMon.VTDecodeUnreliable()
}

// SoftwareDecodeUnreliable reports sustained software decoder corruption in stderr.
func (j *HLSVODSegmentJob) SoftwareDecodeUnreliable() bool {
	if j == nil {
		return false
	}
	return j.stderrMon.SoftwareDecodeUnreliable()
}

func prepareVODOutputLayout(outDir string, opts HLSVODJobOptions) error {
	segDir := filepath.Join(outDir, HLSVODSegmentSubdir)
	if opts.ClearSegmentsFromIndex {
		entries, err := os.ReadDir(segDir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".m4s") {
				continue
			}
			var idx int
			if _, err := fmt.Sscanf(name, "%05d.m4s", &idx); err != nil {
				continue
			}
			if idx >= opts.StartIndex {
				_ = os.Remove(filepath.Join(segDir, name))
			}
		}
	}
	return os.MkdirAll(segDir, 0o755)
}

// BuildHLSVODFFmpegArgs builds frozen argv for a Model A forward segment encode job.
// The host must not mutate these flags; see docs/hls-vod-contract.md.
func BuildHLSVODFFmpegArgs(runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSVODJobOptions) ([]string, error) {
	if opts.SegmentSec <= 0 {
		opts.SegmentSec = DefaultHLSSegmentDurationSec
	}
	segDur := opts.SegmentSec
	gop := opts.GOP
	if gop <= 0 {
		gop = HLSSegmentGOP(defaultHLSSegmentFPS, DefaultOnDemandHLSDefaults())
	}

	args := []string{"-hide_banner", "-nostats", "-loglevel", hlsFFmpegLogLevel(runner), "-y"}
	videoCopyPipeline := opts.Remux || opts.VideoCopy
	inputSeek := vodInputSeekMediaSec(opts)
	inputSeek = HLSInputSeekSec(inputSeek, opts.InputStartTimeSec)
	if inputSeek > hlsStartTimeEpsilonSec {
		args = append(args, "-ss", fmt.Sprintf("%.3f", inputSeek))
	}
	if videoCopyPipeline {
		args = appendHLSInputTimestampFlags(args)
	}
	args = appendInputFlags(args, runner, opts.Input, nil, nil)
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
			audioFilter := vodAudioResampleFilter(opts.StartSec)
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
		args = append(args, "-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%.3f)", segDur))
		if !opts.VideoOnly() {
			audioFilter := vodAudioResampleFilter(opts.StartSec)
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
	}
	// Playlist-local timestamps: one constant offset per job, no per-segment tfdt surgery for transcode.
	args = appendHLSVODOutputTimestampArgs(args, opts.StartSec)

	args = append(args,
		"-max_muxing_queue_size", "128",
		"-f", "hls",
		"-max_delay", "5000000",
		"-hls_time", fmt.Sprintf("%.3f", segDur),
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", HLSVODInitFilename,
		"-hls_segment_filename", HLSVODSegmentNamePattern,
		"-start_number", fmt.Sprintf("%d", opts.StartIndex),
		"-hls_list_size", "0",
		"-hls_flags", "temp_file+independent_segments",
		HLSVODInternalPlaylistName,
	)
	return args, nil
}

func vodInputSeekMediaSec(opts HLSVODJobOptions) float64 {
	mediaSeek := opts.StartSec
	if (opts.Remux || opts.VideoCopy) && len(opts.KeyframeSeekTimes) > 0 {
		mediaSeek = KeyframeSeekBefore(opts.KeyframeSeekTimes, opts.StartSec)
	}
	if mediaSeek < 0 {
		mediaSeek = 0
	}
	return mediaSeek
}

// aresample async=1 is enough for A/V drift; avoid huge async windows (e.g. 192000) that pad silence on seeks.
// appendHLSVODOutputTimestampArgs shifts muxed fMP4 tfdt to the synthetic playlist timeline.
func appendHLSVODOutputTimestampArgs(args []string, startSec float64) []string {
	if startSec > hlsStartTimeEpsilonSec {
		args = append(args, "-avoid_negative_ts", "disabled", "-output_ts_offset", fmt.Sprintf("%.6f", startSec))
		return args
	}
	return append(args, "-avoid_negative_ts", "make_zero")
}

// vodAbsLocalInputURL applies filepath.Abs only to local paths (not http/rtsp URLs).
func vodAbsLocalInputURL(url string) string {
	if url == "" || strings.Contains(url, "://") {
		return url
	}
	if abs, err := filepath.Abs(url); err == nil {
		return abs
	}
	return url
}

func vodAudioResampleFilter(startSec float64) string {
	if startSec <= hlsStartTimeEpsilonSec {
		return "aresample=async=1:first_pts=0"
	}
	return "aresample=async=1"
}

// StartHLSVODSegmentJob launches ffmpeg writing fMP4 segments for on-demand VOD Model A.
func StartHLSVODSegmentJob(parent context.Context, runner *ffexec.Runner, caps *capabilities.Capabilities, opts HLSVODJobOptions) (*HLSVODSegmentJob, error) {
	if runner == nil || caps == nil {
		return nil, fmt.Errorf("hls vod: runner or capabilities missing")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return nil, fmt.Errorf("hls vod: output dir required")
	}
	if opts.SegmentSec <= 0 {
		opts.SegmentSec = DefaultHLSSegmentDurationSec
	}
	outDir, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("hls vod: output dir: %w", err)
	}
	opts.OutputDir = outDir
	inputURL := vodAbsLocalInputURL(opts.Input.URL)
	if inputURL != opts.Input.URL {
		opts.Input = InputSource{URL: inputURL, StreamType: opts.Input.StreamType}
	}
	if err := prepareVODOutputLayout(outDir, opts); err != nil {
		return nil, err
	}

	args, err := BuildHLSVODFFmpegArgs(runner, caps, opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(parent)
	cmd := ffexec.CommandContext(ctx, runner.FFmpegPath, args...)
	cmd.Dir = outDir
	stderr := newStderrTail(continuousStderrTailMax)
	var dst io.Writer = stderr
	if runner.VerboseFFmpeg {
		dst = io.MultiWriter(os.Stderr, stderr)
	}
	stderrMon := newVTDecodeStderrMonitor(dst)
	cmd.Stderr = stderrMon

	job := &HLSVODSegmentJob{
		ffmpegDone: make(chan struct{}),
		stop:       cancel,
		stderrMon:  stderrMon,
		startSec:   opts.StartSec,
		startIndex: opts.StartIndex,
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
		cancel()
		job.mu.Lock()
		job.err = err
		job.mu.Unlock()
		close(job.ffmpegDone)
	}()
	return job, nil
}

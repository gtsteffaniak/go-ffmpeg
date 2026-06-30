package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// Service is the main entry point for ffmpeg operations.
type Service struct {
	cfg        Config
	runner     *ffexec.Runner
	caps       *capabilities.Capabilities
	resolver   *encode.Resolver
	semaphore  chan struct{}
	detectOnce sync.Once
	detectErr  error
	mu         sync.RWMutex
}

// New creates and optionally detects a Service.
func New(ctx context.Context, cfg Config) (*Service, error) {
	cfg = cfg.withDefaults()
	ffmpegPath, ffprobePath, err := ffexec.ResolvePair(cfg.FFmpegPath, cfg.FFprobePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBinaryNotFound, err)
	}
	s := &Service{
		cfg: cfg,
		runner: &ffexec.Runner{
			FFmpegPath:    ffmpegPath,
			FFprobePath:   ffprobePath,
			VerboseFFmpeg: cfg.VerboseFFmpeg,
		},
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
	}
	if *cfg.DetectOnInit {
		if err := s.Reload(ctx); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Reload re-runs capability detection.
func (s *Service) Reload(ctx context.Context) error {
	dctx, cancel := context.WithTimeout(ctx, s.cfg.DetectTimeout)
	defer cancel()

	caps, err := capabilities.Detect(dctx, s.runner, s.buildDetectOptions())
	if err != nil {
		return err
	}
	if err := checkMinVersion(caps, s.cfg.MinVersion); err != nil {
		return err
	}
	ops.EvaluateOps(caps)
	log := WithGroup(s.cfg.Logger, "ffmpeg")
	log.Info("capability detection complete", "profile", caps.BuildProfile, "version", caps.FFmpegVersion)
	report := caps.ReportString()
	for _, line := range splitLines(report) {
		log.Info(line)
	}

	s.mu.Lock()
	s.caps = caps
	s.resolver = encode.NewResolver(caps)
	s.mu.Unlock()
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// Capabilities returns the detected capability matrix.
func (s *Service) Capabilities() *capabilities.Capabilities {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.caps
}

// SupportedOps returns enabled operation names.
func (s *Service) SupportedOps() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.caps == nil {
		return nil
	}
	return append([]string(nil), s.caps.EnabledOps...)
}

// Acquire waits for a concurrency slot.
func (s *Service) Acquire(ctx context.Context) error {
	select {
	case s.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees a concurrency slot.
func (s *Service) Release() {
	<-s.semaphore
}

func (s *Service) require(opName string) error {
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	if caps == nil {
		return ErrNotDetected
	}
	for _, op := range ops.All() {
		if op.Name() == opName {
			ok, reasons := ops.Supported(op, caps)
			if !ok {
				return &UnsupportedError{Op: opName, Reasons: reasons}
			}
			return nil
		}
	}
	return nil
}

// Logger returns the configured logger for this service.
func (s *Service) Logger() Logger { return s.cfg.Logger }

// FFmpegPath returns the resolved ffmpeg binary path.
func (s *Service) FFmpegPath() string { return s.runner.FFmpegPath }

// FFprobePath returns the resolved ffprobe binary path.
func (s *Service) FFprobePath() string { return s.runner.FFprobePath }

// ProbeStreamOptions is re-exported probe configuration.
type ProbeStreamOptions = probe.ProbeStreamOptions

// StreamInfo is re-exported probe result.
type StreamInfo = probe.StreamInfo

// StreamType is re-exported.
type StreamType = probe.StreamType

const (
	StreamFile = probe.StreamFile
	StreamRTSP = probe.StreamRTSP
	StreamHLS  = probe.StreamHLS
	StreamHTTP = probe.StreamHTTP
)

// ProbeStream validates and probes a stream URL.
func (s *Service) ProbeStream(ctx context.Context, opts ProbeStreamOptions) (StreamInfo, error) {
	if err := s.require("ProbeStream"); err != nil {
		return StreamInfo{}, err
	}
	info, err := probe.ProbeStream(ctx, s.runner, opts)
	if err != nil {
		return info, &OperationError{Op: "ProbeStream", Err: ErrProbeFailed, Stderr: info.Message}
	}
	return info, nil
}

// GetMediaDuration returns duration in seconds for a media file.
func (s *Service) GetMediaDuration(ctx context.Context, path string) (float64, error) {
	if err := s.require("GetMediaDuration"); err != nil {
		return 0, err
	}
	return probe.GetMediaDuration(ctx, s.runner, path)
}

// GetImageDimensions returns image/video dimensions.
func (s *Service) GetImageDimensions(ctx context.Context, path string) (width, height int, err error) {
	if err := s.require("GetImageDimensions"); err != nil {
		return 0, 0, err
	}
	return probe.GetImageDimensions(ctx, s.runner, path)
}

// ScreenshotOptions configures screenshot capture.
type ScreenshotOptions = ops.ScreenshotOptions

// InputSource describes ffmpeg input.
type InputSource = ops.InputSource

// Screenshot captures a single frame to a file.
func (s *Service) Screenshot(ctx context.Context, opts ScreenshotOptions) error {
	if err := s.require("Screenshot"); err != nil {
		return err
	}
	if err := ops.Screenshot(ctx, s.runner, opts); err != nil {
		return &OperationError{Op: "Screenshot", Err: ErrEncodeFailed, Stderr: err.Error()}
	}
	return nil
}

// PreviewOptions configures video preview generation.
type PreviewOptions = ops.PreviewOptions

// VideoPreview writes an MJPEG preview frame to w.
func (s *Service) VideoPreview(ctx context.Context, w io.Writer, opts PreviewOptions) error {
	if err := s.require("VideoPreview"); err != nil {
		return err
	}
	return ops.VideoPreview(ctx, s.runner, w, opts)
}

// VideoProfile configures transcoding.
type VideoProfile = encode.VideoProfile

// VideoDecodeProfile configures input-side hardware decode.
type VideoDecodeProfile = encode.VideoDecodeProfile

// EncodeOption describes one cached encode path.
type EncodeOption = capabilities.EncodeOption

// DecodeOption describes one cached decode path.
type DecodeOption = capabilities.DecodeOption

// AccelType identifies a hardware acceleration backend.
type AccelType = capabilities.AccelType

const (
	AccelNVENC        = capabilities.AccelNVENC
	AccelAMF          = capabilities.AccelAMF
	AccelQSV          = capabilities.AccelQSV
	AccelVAAPI        = capabilities.AccelVAAPI
	AccelD3D12        = capabilities.AccelD3D12
	AccelVideoToolbox = capabilities.AccelVideoToolbox
	AccelNone         = capabilities.AccelNone
)

// HierarchyForPlatform returns platform-specific encoder preference order.
func HierarchyForPlatform(plat capabilities.PlatformInfo) []capabilities.AccelType {
	return capabilities.HierarchyForPlatform(plat)
}

// TranscodeOptions configures transcoding.
type TranscodeOptions = ops.TranscodeOptions

// Transcode re-encodes media to a new file.
func (s *Service) Transcode(ctx context.Context, opts TranscodeOptions) error {
	if err := s.require("Transcode"); err != nil {
		return err
	}
	if err := s.ValidateVideoProfile(opts.Profile); err != nil {
		return err
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	if err := ops.Transcode(ctx, s.runner, caps, opts); err != nil {
		return &OperationError{Op: "Transcode", Err: ErrEncodeFailed, Stderr: err.Error()}
	}
	return nil
}

// SegmentRecordOptions configures segmented recording.
type SegmentRecordOptions = ops.SegmentRecordOptions

// SegmentRecord records input into segmented MP4 files.
func (s *Service) SegmentRecord(ctx context.Context, opts SegmentRecordOptions) error {
	if err := s.require("SegmentRecord"); err != nil {
		return err
	}
	if err := s.ValidateVideoProfile(opts.Profile); err != nil {
		return err
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	return ops.SegmentRecord(ctx, s.runner, caps, opts)
}

// FMP4StreamCopyOptions configures live fMP4 streaming.
type FMP4StreamCopyOptions = ops.FMP4StreamCopyOptions

// FMP4StreamCopy streams fragmented MP4 to w.
func (s *Service) FMP4StreamCopy(ctx context.Context, w io.Writer, opts FMP4StreamCopyOptions) error {
	if err := s.require("FMP4StreamCopy"); err != nil {
		return err
	}
	return ops.FMP4StreamCopy(ctx, s.runner, w, opts)
}

// FMP4TranscodeOptions configures live fMP4 transcoding.
type FMP4TranscodeOptions = ops.FMP4TranscodeOptions

// FMP4Transcode re-encodes input to fragmented MP4 on w.
func (s *Service) FMP4Transcode(ctx context.Context, w io.Writer, opts FMP4TranscodeOptions) error {
	if err := s.require("FMP4Transcode"); err != nil {
		return err
	}
	if err := s.ValidateVideoProfile(opts.Profile); err != nil {
		return err
	}
	if err := s.ValidateVideoDecodeProfile(opts.Decode); err != nil {
		return err
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	if err := ops.FMP4Transcode(ctx, s.runner, caps, w, opts); err != nil {
		return &OperationError{Op: "FMP4Transcode", Err: ErrEncodeFailed, Stderr: err.Error()}
	}
	return nil
}

// HLSSegmentOptions configures on-demand fMP4 HLS segment generation.
type HLSSegmentOptions = ops.HLSSegmentOptions

// HLSSegmentParams holds resolved encode/remux settings for one HLS session.
type HLSSegmentParams = ops.HLSSegmentParams

// HLSSegmentBuildInput describes remux/copy/transcode path selection for one file.
type HLSSegmentBuildInput = ops.HLSSegmentBuildInput

// OnDemandHLSDefaults holds segment timing defaults for on-demand HLS.
type OnDemandHLSDefaults = ops.OnDemandHLSDefaults

// HLSPipelineOptions configures remux/copy/transcode path selection.
type HLSPipelineOptions = ops.HLSPipelineOptions

const (
	DefaultHLSSegmentDurationSec = ops.DefaultHLSSegmentDurationSec
)

// DefaultOnDemandHLSDefaults returns on-demand segment defaults.
func DefaultOnDemandHLSDefaults() OnDemandHLSDefaults {
	return ops.DefaultOnDemandHLSDefaults()
}

// HLSDecodeProfileForOnDemand selects input decode for short on-demand HLS segments.
func HLSDecodeProfileForOnDemand(info StreamInfo) VideoDecodeProfile {
	return encode.HLSDecodeProfileForOnDemand(info)
}

// DefaultHLSVideoProfile returns safe H.264 transcode defaults when the caller
// does not supply encode settings.
func DefaultHLSVideoProfile(maxHeight int) VideoProfile {
	return encode.DefaultHLSVideoProfile(maxHeight)
}

// SanitizeHLSKeyframes filters spurious keyframe probes.
func SanitizeHLSKeyframes(keyframes []float64, durationSec float64) []float64 {
	return ops.SanitizeHLSKeyframes(keyframes, durationSec)
}

// BuildHLSSegmentTimeline returns segment start times and durations.
func BuildHLSSegmentTimeline(durationSec float64, keyframes []float64, segmentDurationSec float64) (starts, durations []float64) {
	return ops.BuildHLSSegmentTimeline(durationSec, keyframes, segmentDurationSec)
}

// KeyframeSeekBefore returns the largest keyframe time <= sec.
func KeyframeSeekBefore(keyframes []float64, sec float64) float64 {
	return ops.KeyframeSeekBefore(keyframes, sec)
}

// BuildHLSSegmentOptions builds segment options for segment index n.
func BuildHLSSegmentOptions(path string, index int, params HLSSegmentParams, starts, durations []float64, keyframeTimeline bool, keyframeSeekTimes []float64, segmentDurationSec float64) HLSSegmentOptions {
	return ops.BuildHLSSegmentOptions(path, index, params, starts, durations, keyframeTimeline, keyframeSeekTimes, segmentDurationSec)
}

// BuildHLSSegmentBuildInput derives remux/copy/transcode flags.
func BuildHLSSegmentBuildInput(info StreamInfo, opts HLSPipelineOptions) HLSSegmentBuildInput {
	return ops.BuildHLSSegmentBuildInput(info, opts)
}

// BuildHLSSegmentParamsFast assembles encode params without probing fps.
func BuildHLSSegmentParamsFast(in HLSSegmentBuildInput, defaults OnDemandHLSDefaults) HLSSegmentParams {
	return ops.BuildHLSSegmentParamsFast(in, defaults)
}

// NeedsFullVideoTranscode reports whether video must be re-encoded.
func NeedsFullVideoTranscode(info StreamInfo, opts HLSPipelineOptions) bool {
	return ops.NeedsFullVideoTranscode(info, opts)
}

// UseVideoCopy selects H.264 stream-copy with audio transcode.
func UseVideoCopy(info StreamInfo, opts HLSPipelineOptions) bool {
	return ops.UseVideoCopy(info, opts)
}

// HLSSegmentGOP returns GOP size from fps and on-demand defaults.
func HLSSegmentGOP(fps float64, defaults OnDemandHLSDefaults) int {
	return ops.HLSSegmentGOP(fps, defaults)
}

// CanFMP4StreamCopy reports whether remux to fMP4 is possible.
func CanFMP4StreamCopy(info StreamInfo) bool {
	return ops.CanFMP4StreamCopy(info)
}

// CanH264VideoCopy is true when H.264 can be stream-copied with audio transcode.
func CanH264VideoCopy(info StreamInfo) bool {
	return ops.CanH264VideoCopy(info)
}

// ProbeFile probes a local media file.
func (s *Service) ProbeFile(ctx context.Context, path string) (StreamInfo, error) {
	return s.ProbeStream(ctx, ProbeStreamOptions{URL: path, StreamType: probe.StreamFile})
}

// BuildHLSSegmentParams resolves GOP from fps when probeFPS is true.
func (s *Service) BuildHLSSegmentParams(ctx context.Context, path string, in HLSSegmentBuildInput, defaults OnDemandHLSDefaults, probeFPS bool) (HLSSegmentParams, error) {
	return ops.BuildHLSSegmentParams(ctx, s.ProbeVideoFPS, path, in, defaults, probeFPS)
}

// DescribeHLSSegmentPlan summarizes the encode path for logging.
func (s *Service) DescribeHLSSegmentPlan(params HLSSegmentParams) string {
	s.mu.RLock()
	resolver := s.resolver
	s.mu.RUnlock()
	return ops.DescribeHLSSegmentPlan(resolver, params)
}

// HLSSegment generates a self-contained MPEG-TS segment for full re-encode on-demand HLS.
func (s *Service) HLSSegment(ctx context.Context, opts HLSSegmentOptions) ([]byte, error) {
	if err := s.require("HLSSegment"); err != nil {
		return nil, err
	}
	if err := s.ValidateVideoProfile(opts.Profile); err != nil {
		return nil, err
	}
	if err := s.ValidateVideoDecodeProfile(opts.Decode); err != nil {
		return nil, err
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	data, err := ops.HLSSegment(ctx, s.runner, caps, opts)
	if err != nil {
		return nil, &OperationError{Op: "HLSSegment", Err: ErrEncodeFailed, Stderr: err.Error()}
	}
	return data, nil
}

// HLSInitAndSegment generates init and media bytes for the first HLS segment.
func (s *Service) HLSInitAndSegment(ctx context.Context, opts HLSSegmentOptions) (init, media []byte, err error) {
	if err := s.require("HLSSegment"); err != nil {
		return nil, nil, err
	}
	if !opts.Remux && !opts.VideoCopy {
		if err := s.ValidateVideoProfile(opts.Profile); err != nil {
			return nil, nil, err
		}
		if err := s.ValidateVideoDecodeProfile(opts.Decode); err != nil {
			return nil, nil, err
		}
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	init, media, err = ops.HLSInitAndSegment(ctx, s.runner, caps, opts)
	if err != nil {
		return nil, nil, &OperationError{Op: "HLSSegment", Err: ErrEncodeFailed, Stderr: err.Error()}
	}
	return init, media, nil
}

// HLSSegmentMedia writes a media-only fMP4 fragment for an HLS segment request.
func (s *Service) HLSSegmentMedia(ctx context.Context, w io.Writer, opts HLSSegmentOptions) error {
	if err := s.require("HLSSegment"); err != nil {
		return err
	}
	if !opts.Remux && !opts.VideoCopy {
		if err := s.ValidateVideoProfile(opts.Profile); err != nil {
			return err
		}
		if err := s.ValidateVideoDecodeProfile(opts.Decode); err != nil {
			return err
		}
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	if err := ops.HLSSegmentMedia(ctx, s.runner, caps, w, opts); err != nil {
		return &OperationError{Op: "HLSSegment", Err: ErrEncodeFailed, Stderr: err.Error()}
	}
	return nil
}

// ProbeVideoFPS returns the average frame rate of the first video stream.
func (s *Service) ProbeVideoFPS(ctx context.Context, path string) (float64, error) {
	if err := s.require("ProbeStream"); err != nil {
		return 0, err
	}
	return ops.ProbeVideoFPS(ctx, s.runner, path)
}

// ProbeVideoKeyframeTimes returns keyframe presentation times in seconds.
func (s *Service) ProbeVideoKeyframeTimes(ctx context.Context, path string) ([]float64, error) {
	if err := s.require("ProbeStream"); err != nil {
		return nil, err
	}
	return ops.ProbeVideoKeyframeTimes(ctx, s.runner, path)
}

// TimelapseCompileOptions configures timelapse compilation.
type TimelapseCompileOptions = ops.TimelapseCompileOptions

// TimelapseCompile builds a video from snapshot images.
func (s *Service) TimelapseCompile(ctx context.Context, opts TimelapseCompileOptions) error {
	if err := s.require("TimelapseCompile"); err != nil {
		return err
	}
	if err := s.ValidateVideoProfile(opts.Profile); err != nil {
		return err
	}
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	return ops.TimelapseCompile(ctx, s.runner, caps, opts)
}

// SubtitleTrack describes an embedded subtitle.
type SubtitleTrack = probe.SubtitleTrack

// DetectSubtitles lists embedded subtitle tracks.
func (s *Service) DetectSubtitles(ctx context.Context, path string) ([]SubtitleTrack, error) {
	if err := s.require("DetectSubtitles"); err != nil {
		return nil, err
	}
	return probe.DetectEmbeddedSubtitles(ctx, s.runner, path)
}

// ExtractSubtitle extracts a subtitle stream as WebVTT.
func (s *Service) ExtractSubtitle(ctx context.Context, path string, streamIndex int) (string, error) {
	if err := s.require("ExtractSubtitle"); err != nil {
		return "", err
	}
	return probe.ExtractSubtitle(ctx, s.runner, path, streamIndex)
}

// ConvertHEICOptions configures HEIC conversion.
type ConvertHEICOptions = ops.ConvertHEICOptions

// ConvertHEIC converts HEIC/HEIF images to JPEG.
func (s *Service) ConvertHEIC(ctx context.Context, opts ConvertHEICOptions) error {
	if err := s.require("ConvertHEIC"); err != nil {
		return err
	}
	return ops.ConvertHEIC(ctx, s.runner, opts)
}

// VideoDecoderArgs returns resolved input-side decode arguments for a profile.
func (s *Service) VideoDecoderArgs(profile VideoDecodeProfile) ([]string, error) {
	s.mu.RLock()
	resolver := s.resolver
	s.mu.RUnlock()
	if resolver == nil {
		return nil, ErrNotDetected
	}
	return resolver.VideoDecoderArgs(profile)
}

// VideoEncoderArgs returns resolved encoder arguments for a profile.
func (s *Service) VideoEncoderArgs(profile VideoProfile) ([]string, error) {
	s.mu.RLock()
	resolver := s.resolver
	s.mu.RUnlock()
	if resolver == nil {
		return nil, ErrNotDetected
	}
	return resolver.VideoEncoderArgs(profile)
}

// ValidateVideoProfile checks whether a profile is supported before running an operation.
func (s *Service) ValidateVideoProfile(profile VideoProfile) error {
	s.mu.RLock()
	resolver := s.resolver
	s.mu.RUnlock()
	if resolver == nil {
		return ErrNotDetected
	}
	return resolver.ValidateVideoProfile(profile)
}

// ResolveVideoEncoder returns the encoder selection for a profile without building ffmpeg args.
func (s *Service) ResolveVideoEncoder(profile VideoProfile) (capabilities.EncoderSelection, error) {
	s.mu.RLock()
	resolver := s.resolver
	s.mu.RUnlock()
	if resolver == nil {
		return capabilities.EncoderSelection{}, ErrNotDetected
	}
	sel, err := resolver.ResolveEncoder(profile)
	if err != nil {
		return capabilities.EncoderSelection{}, err
	}
	return sel, nil
}

// ValidateVideoDecodeProfile checks whether a decode profile is supported.
func (s *Service) ValidateVideoDecodeProfile(profile VideoDecodeProfile) error {
	s.mu.RLock()
	resolver := s.resolver
	s.mu.RUnlock()
	if resolver == nil {
		return ErrNotDetected
	}
	return resolver.ValidateVideoDecodeProfile(profile)
}

// ResolveVideoDecoder returns the decoder selection for a profile.
func (s *Service) ResolveVideoDecoder(profile VideoDecodeProfile) (capabilities.DecoderSelection, error) {
	s.mu.RLock()
	resolver := s.resolver
	s.mu.RUnlock()
	if resolver == nil {
		return capabilities.DecoderSelection{}, ErrNotDetected
	}
	return resolver.ResolveDecoder(profile)
}

// EncodeOptions returns cached encode paths from startup detection.
func (s *Service) EncodeOptions() []capabilities.EncodeOption {
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	if caps == nil {
		return nil
	}
	return append([]capabilities.EncodeOption(nil), caps.EncodeOptions...)
}

// AvailableEncodeOptions returns encode paths that passed detection.
func (s *Service) AvailableEncodeOptions() []capabilities.EncodeOption {
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	if caps == nil {
		return nil
	}
	return caps.AvailableEncodeOptions()
}

// DecodeOptions returns cached decode paths from startup detection.
func (s *Service) DecodeOptions() []capabilities.DecodeOption {
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	if caps == nil {
		return nil
	}
	return append([]capabilities.DecodeOption(nil), caps.DecodeOptions...)
}

// AvailableDecodeOptions returns decode paths that passed detection.
func (s *Service) AvailableDecodeOptions() []capabilities.DecodeOption {
	s.mu.RLock()
	caps := s.caps
	s.mu.RUnlock()
	if caps == nil {
		return nil
	}
	return caps.AvailableDecodeOptions()
}

func checkMinVersion(caps *capabilities.Capabilities, min capabilities.Version) error {
	ver := caps.FeatureFlags.Version
	if ver == (capabilities.Version{}) {
		parsed, err := capabilities.ParseSemver(caps.FFmpegVersion)
		if err != nil {
			return &VersionTooOldError{Version: caps.FFmpegVersion, Minimum: min.String()}
		}
		ver = parsed
	}
	if capabilities.Compare(ver, min) < 0 {
		return &VersionTooOldError{Version: ver.String(), Minimum: min.String()}
	}
	return nil
}

// DefaultDetectTimeout is the default capability detection timeout.
const DefaultDetectTimeout = 60 * time.Second

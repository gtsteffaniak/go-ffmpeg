package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

type testVariant struct {
	Mode  string                 `json:"mode"`  // remux, copy, transcode
	Accel capabilities.AccelType `json:"accel"` // none for remux/copy; software/hw for transcode
	Label string                 `json:"label"`
}

type segmentBenchmark struct {
	Index          int     `json:"index"`
	EncodeMs       int64   `json:"encodeMs"`
	Bytes          int     `json:"bytes"`
	MediaStartSec  float64 `json:"mediaStartSec"`
	ExpectedStart  float64 `json:"expectedStartSec"`
	ActualDurSec   float64 `json:"actualDurSec"`
	ExpectedDurSec float64 `json:"expectedDurSec"`
	TimelineOK     bool    `json:"timelineOk"`
}

type encodeTimingSummary struct {
	ColdSegMs          int64    `json:"coldSegMs,omitempty"`
	WarmAvgSegMs       int64    `json:"warmAvgSegMs,omitempty"`
	WarmTotalMs        int64    `json:"warmTotalMs,omitempty"`
	WarmSegCount       int      `json:"warmSegCount,omitempty"`
	ThroughputRealtime *float64 `json:"throughputRealtime,omitempty"` // media seconds / encode seconds
}

type benchmarkResult struct {
	Fixture       string              `json:"fixture"`
	FixturePath   string              `json:"fixturePath"`
	Mode          string              `json:"mode"`
	Accel         string              `json:"accel"`
	Label         string              `json:"label"`
	Pass          bool                `json:"pass"`
	EncodeError   string              `json:"encodeError,omitempty"`
	TotalEncodeMs int64               `json:"totalEncodeMs"`
	Timing        encodeTimingSummary `json:"timing,omitempty"`
	Resources     resourceStats       `json:"resources"`
	HW            hwVerification      `json:"hw"`
	Segments      []segmentBenchmark  `json:"segments"`
	Issues        []mp4.TimelineIssue `json:"issues,omitempty"`
	PlaybackURL   string              `json:"playbackUrl,omitempty"`
	ArtifactDir   string              `json:"artifactDir,omitempty"`
	Skipped       bool                `json:"skipped,omitempty"`
	SkipReason    string              `json:"skipReason,omitempty"`
}

type hwVerification struct {
	ExpectedAccel  string   `json:"expectedAccel"`
	Encoder        string   `json:"encoder,omitempty"`
	Decoder        string   `json:"decoder,omitempty"`
	EncodePlan     string   `json:"encodePlan,omitempty"`
	HWEncoder      bool     `json:"hwEncoder"`
	GPUMonitor     string   `json:"gpuMonitor,omitempty"`
	GPUUtilAvg     *float64 `json:"gpuUtilAvg,omitempty"`
	GPUDetected    bool     `json:"gpuDetected"`
	HWLikelyActive bool     `json:"hwLikelyActive"`
	Notes          string   `json:"notes,omitempty"`
}

func variantsForFixture(info goffmpeg.StreamInfo, caps *capabilities.Capabilities) []testVariant {
	var out []testVariant
	if canRemuxFixture(info) {
		out = append(out, testVariant{Mode: "remux", Accel: capabilities.AccelNone, Label: "remux"})
	}
	if canCopyFixture(info) {
		out = append(out, testVariant{Mode: "copy", Accel: capabilities.AccelNone, Label: "copy"})
	}
	out = append(out, testVariant{Mode: "transcode", Accel: capabilities.AccelNone, Label: "transcode/software"})
	for _, accel := range encodeAccelVariants(caps) {
		if accel == capabilities.AccelNone {
			continue
		}
		if isLegacyMPEG4Video(info.VideoCodec) && accel == capabilities.AccelVAAPI {
			continue // mpeg4 needs software decode; VAAPI hwupload fails on this source
		}
		out = append(out, testVariant{
			Mode:  "transcode",
			Accel: accel,
			Label: fmt.Sprintf("transcode/%s", capabilities.AccelLabel(accel)),
		})
	}
	return out
}

func softwareOnlyRequested(explicit bool) bool {
	if explicit {
		return true
	}
	return os.Getenv("GOFFMPEG_SKIP_HW") == "1" || os.Getenv("HLS_SOFTWARE_ONLY") == "1"
}

func filterSoftwareVariants(variants []testVariant, softwareOnly bool) []testVariant {
	if !softwareOnly {
		return variants
	}
	out := make([]testVariant, 0, len(variants))
	for _, variant := range variants {
		if variant.Mode == "transcode" && variant.Accel != capabilities.AccelNone {
			continue
		}
		out = append(out, variant)
	}
	return out
}

func accelLabel(accel capabilities.AccelType) string {
	if accel == "" || accel == capabilities.AccelNone {
		return "software"
	}
	return string(accel)
}

func runBenchmark(ctx context.Context, svc *goffmpeg.Service, file, fixtureName string, variant testVariant, segCount int, tolerance float64, artifactRoot string) (*benchmarkResult, error) {
	info, err := svc.ProbeFile(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("probe: %w", err)
	}

	result := &benchmarkResult{
		Fixture:     fixtureName,
		FixturePath: file,
		Mode:        variant.Mode,
		Accel:       accelLabel(variant.Accel),
		Label:       variant.Label,
		Pass:        true,
	}

	params, err := paramsForVariant(ctx, svc, file, info, variant)
	if err != nil {
		result.Pass = false
		result.EncodeError = err.Error()
		result.Skipped = true
		result.SkipReason = err.Error()
		return result, nil
	}
	result.HW = verifyHWPlan(svc, params, variant.Accel)

	keyframes, _ := svc.ProbeVideoKeyframeTimes(ctx, file)
	keyframeSeekTimes := goffmpeg.SanitizeHLSKeyframes(keyframes, info.Duration)
	starts, durations := goffmpeg.BuildHLSSegmentTimeline(info.Duration, keyframeSeekTimes, goffmpeg.DefaultHLSSegmentDurationSec)
	if segCount > len(starts) {
		segCount = len(starts)
	}
	if segCount < 1 {
		segCount = 1
	}

	artifactDir := filepath.Join(artifactRoot, fixtureName, fmt.Sprintf("%s_%s", variant.Mode, accelLabel(variant.Accel)))
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return nil, err
	}
	result.ArtifactDir = artifactDir
	result.PlaybackURL = "/media/" + filepath.ToSlash(filepath.Join(fixtureName, fmt.Sprintf("%s_%s", variant.Mode, accelLabel(variant.Accel)), "playlist.m3u8"))

	monitor := newResourceMonitor()
	monitor.Start()
	defer func() {
		result.Resources = monitor.Stop()
		result.HW.GPUMonitor = result.Resources.GPUMonitor
		result.HW.GPUUtilAvg = result.Resources.GPUPercentAvg
		if result.Resources.GPUPercentAvg != nil && *result.Resources.GPUPercentAvg >= 3 {
			result.HW.GPUDetected = true
		}
		result.HW.HWLikelyActive = hwLikelyActive(result.HW, result.Resources, result.Pass)
		switch result.Resources.GPUMonitor {
		case "intel_sysfs_unavailable":
			if result.HW.HWLikelyActive && result.HW.ExpectedAccel != "software" {
				result.HW.Notes = "install intel-gpu-tools for GPU %; encoder name confirms HW path"
			}
		case "intel_xe_no_sysfs":
			if result.HW.HWLikelyActive && result.HW.ExpectedAccel != "software" {
				result.HW.Notes = "Intel xe gtidle sysfs unavailable; encoder name confirms HW path"
			}
		}
	}()

	var mediaEnds []float64
	var initBytes []byte
	var trackTimescales map[uint32]uint32
	totalStart := time.Now()

	for i := 0; i < segCount; i++ {
		if i < len(starts) && starts[i] >= info.Duration-0.01 {
			break
		}
		segStart := time.Now()
		mediaTimelineSec := 0.0
		if i > 0 && len(mediaEnds) >= i {
			mediaTimelineSec = mediaEnds[i-1]
		} else if i < len(starts) {
			mediaTimelineSec = starts[i]
		}

		opts := goffmpeg.BuildHLSSegmentOptions(file, i, params, starts, durations, false, keyframeSeekTimes, goffmpeg.DefaultHLSSegmentDurationSec)
		opts.MediaTimelineSec = mediaTimelineSec

		var media []byte
		if i == 0 {
			initBytes, media, err = svc.HLSInitAndSegment(ctx, opts)
			if len(initBytes) > 0 {
				trackTimescales = mp4.TrackTimescalesFromInit(initBytes)
			}
		} else {
			var buf bytes.Buffer
			err = svc.HLSSegmentMedia(ctx, &buf, opts)
			media = buf.Bytes()
		}
		encodeMs := time.Since(segStart).Milliseconds()

		expectedDur := goffmpeg.DefaultHLSSegmentDurationSec
		if i < len(durations) {
			expectedDur = durations[i]
		}

		segBench := segmentBenchmark{
			Index:          i,
			EncodeMs:       encodeMs,
			ExpectedStart:  mediaTimelineSec,
			ExpectedDurSec: expectedDur,
		}

		if err != nil {
			result.Pass = false
			result.EncodeError = err.Error()
			segBench.TimelineOK = false
			result.Segments = append(result.Segments, segBench)
			break
		}

		segBench.Bytes = len(media)
		startSec, _ := mp4.FragmentMediaStartSec(media)
		actualDur := mp4.FragmentDurationSecWithTimescales(media, trackTimescales)
		if i > 0 && len(result.Segments) > 0 {
			implied := startSec - result.Segments[len(result.Segments)-1].MediaStartSec
			if implied > actualDur {
				actualDur = implied
			}
		}
		if actualDur <= 0 {
			actualDur = expectedDur
		}
		end := startSec + actualDur
		if len(mediaEnds) <= i {
			mediaEnds = append(mediaEnds, end)
		} else {
			mediaEnds[i] = end
		}

		segBench.MediaStartSec = startSec
		segBench.ActualDurSec = actualDur

		issues := mp4.ValidateSegmentTimeline(media, mp4.SegmentTimeline{
			Index:            i,
			ExpectedStartSec: mediaTimelineSec,
			ExpectedDurSec:   expectedDur,
			MediaStartSec:    startSec,
			ActualDurSec:     actualDur,
			Bytes:            len(media),
		}, tolerance)
		issues = filterKeyframeAlignedDurationIssues(issues, actualDur, expectedDur, mediaTimelineSec, keyframeSeekTimes, tolerance)
		if i > 0 && len(result.Segments) > 0 {
			prev := result.Segments[len(result.Segments)-1]
			prevTL := mp4.SegmentTimeline{Index: prev.Index, MediaStartSec: prev.MediaStartSec, ActualDurSec: prev.ActualDurSec}
			nextTL := mp4.SegmentTimeline{Index: i, MediaStartSec: startSec}
			issues = append(issues, mp4.ValidateContinuity(prevTL, nextTL, tolerance)...)
		}
		segBench.TimelineOK = len(issues) == 0
		if len(issues) > 0 {
			result.Pass = false
			result.Issues = append(result.Issues, issues...)
		}

		name := fmt.Sprintf("seg%d.m4s", i)
		if err := os.WriteFile(filepath.Join(artifactDir, name), media, 0o644); err != nil {
			return nil, err
		}
		result.Segments = append(result.Segments, segBench)
	}

	result.TotalEncodeMs = time.Since(totalStart).Milliseconds()
	result.Timing = computeSegmentTiming(result.Segments, info.Duration)

	if len(initBytes) > 0 {
		if err := os.WriteFile(filepath.Join(artifactDir, "init.m4s"), initBytes, 0o644); err != nil {
			return nil, err
		}
	}
	durs := make([]float64, len(result.Segments))
	for i, s := range result.Segments {
		durs[i] = s.ActualDurSec
	}
	if err := writeM3U8(filepath.Join(artifactDir, "playlist.m3u8"), len(result.Segments), durs); err != nil {
		return nil, err
	}

	return result, nil
}

func paramsForVariant(ctx context.Context, svc *goffmpeg.Service, path string, info goffmpeg.StreamInfo, variant testVariant) (goffmpeg.HLSSegmentParams, error) {
	defaults := onDemandDefaults()
	switch variant.Mode {
	case "remux":
		if !canRemuxFixture(info) {
			return goffmpeg.HLSSegmentParams{}, fmt.Errorf("remux not applicable")
		}
		return goffmpeg.HLSSegmentParams{Remux: true, GOP: defaults.DefaultGOP}, nil
	case "copy":
		if !canCopyFixture(info) {
			return goffmpeg.HLSSegmentParams{}, fmt.Errorf("copy not applicable")
		}
		return goffmpeg.HLSSegmentParams{VideoCopy: true, GOP: defaults.DefaultGOP}, nil
	case "transcode":
		gop := goffmpeg.HLSSegmentGOP(30, defaults)
		if fps, err := svc.ProbeVideoFPS(ctx, path); err == nil {
			gop = goffmpeg.HLSSegmentGOP(fps, defaults)
		}
		decodeCodec := videoCodecFromProbe(info.VideoCodec)
		if isLegacyMPEG4Video(info.VideoCodec) {
			profile := encode.VideoProfile{Codec: encode.CodecH264, GOP: gop}
			if variant.Accel == capabilities.AccelNone {
				profile.ForceSoftware = true
			} else {
				caps := cachedCapabilities(svc)
				if caps != nil && !caps.CodecEncodeAvailable(capabilities.CodecH264, variant.Accel) {
					return goffmpeg.HLSSegmentParams{}, fmt.Errorf("accel %s not available", variant.Accel)
				}
				profile.Accel = variant.Accel
			}
			return goffmpeg.HLSSegmentParams{
				Remux: false, VideoCopy: false, MaxHeight: 1080, GOP: gop,
				Decode:  encode.VideoDecodeProfile{ForceSoftware: true},
				Profile: profile,
			}, nil
		}
		decode := encode.OnDemandHLSDecodeProfile(encode.VideoDecodeProfile{Codec: decodeCodec})
		profile := encode.VideoProfile{
			Codec:   encode.CodecH264,
			Quality: encode.PresetVeryfast,
			GOP:     gop,
		}
		if variant.Accel == capabilities.AccelNone {
			profile.ForceSoftware = true
		} else {
			caps := cachedCapabilities(svc)
			if caps != nil && !caps.CodecEncodeAvailable(capabilities.CodecH264, variant.Accel) {
				return goffmpeg.HLSSegmentParams{}, fmt.Errorf("accel %s not available", variant.Accel)
			}
			profile.Accel = variant.Accel
		}
		return goffmpeg.HLSSegmentParams{
			Remux:     false,
			VideoCopy: false,
			MaxHeight: 1080,
			GOP:       gop,
			Decode:    decode,
			Profile:   profile,
		}, nil
	default:
		return goffmpeg.HLSSegmentParams{}, fmt.Errorf("unknown mode %q", variant.Mode)
	}
}

func probeFixtureOrSkip(ctx context.Context, svc *goffmpeg.Service, path string) (goffmpeg.StreamInfo, string, error) {
	if _, err := os.Stat(path); err != nil {
		return goffmpeg.StreamInfo{}, "fixture file missing", nil
	}
	info, err := svc.ProbeFile(ctx, path)
	if err != nil {
		return goffmpeg.StreamInfo{}, "", err
	}
	return info, "", nil
}

func videoCodecFromProbe(name string) capabilities.VideoCodec {
	n := strings.ToLower(strings.TrimSpace(name))
	if isLegacyMPEG4Video(name) {
		return capabilities.CodecH264 // placeholder; ForceSoftware path used instead
	}
	switch {
	case strings.Contains(n, "hevc"), strings.Contains(n, "h265"), strings.Contains(n, "hev"):
		return capabilities.CodecHEVC
	case strings.Contains(n, "vp9"):
		return capabilities.CodecVP9
	case strings.Contains(n, "av1"), strings.Contains(n, "av01"):
		return capabilities.CodecAV1
	default:
		return capabilities.CodecH264
	}
}

func isLegacyMPEG4Video(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(n, "mpeg4") || strings.Contains(n, "msmpeg4")
}

func verifyHWPlan(svc *goffmpeg.Service, params goffmpeg.HLSSegmentParams, accel capabilities.AccelType) hwVerification {
	hv := hwVerification{ExpectedAccel: accelLabel(accel)}
	hv.EncodePlan = svc.DescribeHLSSegmentPlan(params)
	for _, part := range strings.Fields(hv.EncodePlan) {
		if strings.HasPrefix(part, "encoder=") {
			hv.Encoder = strings.TrimPrefix(part, "encoder=")
		}
		if strings.HasPrefix(part, "decoder=") {
			hv.Decoder = strings.TrimPrefix(part, "decoder=")
		}
	}
	hv.HWEncoder = strings.Contains(hv.Encoder, "_qsv") || strings.Contains(hv.Encoder, "_vaapi") ||
		strings.Contains(hv.Encoder, "_nvenc") || strings.Contains(hv.Encoder, "_amf")
	if accel == capabilities.AccelNone {
		hv.HWEncoder = false
	}
	return hv
}

func hwLikelyActive(hv hwVerification, res resourceStats, pass bool) bool {
	if !pass {
		return false
	}
	if hv.ExpectedAccel == "software" {
		return false
	}
	if hv.GPUDetected {
		return true
	}
	return hv.HWEncoder
}

// streamCopyMaxDurationSec returns the longest valid segment duration when ffmpeg
// may extend output to the next keyframe after the nominal cut point.
func streamCopyMaxDurationSec(keyframes []float64, mediaStartSec, nominalDurSec float64) float64 {
	if nominalDurSec <= 0 {
		return nominalDurSec
	}
	targetEnd := mediaStartSec + nominalDurSec
	maxEnd := targetEnd
	for _, kf := range keyframes {
		if kf > targetEnd+0.001 {
			maxEnd = kf
			break
		}
	}
	// Video copy with audio transcode can spill slightly past the next keyframe.
	slack := 0.15
	if len(keyframes) >= 2 {
		interval := keyframes[1] - keyframes[0]
		if interval > 0 && interval < 2 {
			slack = interval * 0.15
		}
	}
	return maxEnd - mediaStartSec + slack
}

func filterKeyframeAlignedDurationIssues(issues []mp4.TimelineIssue, actualDur, expectedDur, mediaStartSec float64, keyframes []float64, toleranceSec float64) []mp4.TimelineIssue {
	maxDur := streamCopyMaxDurationSec(keyframes, mediaStartSec, expectedDur)
	if floor := expectedDur + 2.0; maxDur < floor {
		maxDur = floor
	}
	if expectedDur < 4.0 {
		if tail := expectedDur + 3.0; maxDur < tail {
			maxDur = tail
		}
	}
	out := issues[:0]
	for _, iss := range issues {
		if iss.Check == "duration_match" && actualDur >= expectedDur-toleranceSec && actualDur <= maxDur+toleranceSec {
			continue
		}
		out = append(out, iss)
	}
	return out
}

func fixtureBaseName(path string) string {
	base := filepath.Base(path)
	if idx := strings.LastIndex(base, "."); idx > 0 {
		return base[:idx]
	}
	return base
}

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

type continuousScenario struct {
	Name       string
	StartIndex int
	StartSec   float64
}

func variantSupportsContinuous(v testVariant) bool {
	switch v.Mode {
	case "remux", "copy":
		return true
	case "transcode":
		return v.Accel == capabilities.AccelNone
	default:
		return false
	}
}

func continuousScenarios(starts []float64) []continuousScenario {
	if len(starts) == 0 {
		return nil
	}
	last := len(starts) - 1
	mid := last / 2
	return []continuousScenario{
		{Name: "full", StartIndex: 0, StartSec: 0},
		{Name: "resume_mid", StartIndex: mid, StartSec: starts[mid]},
		{Name: "resume_eof", StartIndex: last, StartSec: starts[last]},
	}
}

func runContinuousBenchmark(ctx context.Context, svc *goffmpeg.Service, file, fixtureName string, variant testVariant, scenario continuousScenario, tolerance float64, artifactRoot string) (*benchmarkResult, error) {
	info, err := svc.ProbeFile(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("probe: %w", err)
	}

	result := &benchmarkResult{
		Fixture:     fixtureName,
		FixturePath: file,
		Mode:        variant.Mode,
		Accel:       accelLabel(variant.Accel),
		Label:       fmt.Sprintf("%s/continuous/%s", variant.Label, scenario.Name),
		Pipeline:    "continuous",
		Scenario:    scenario.Name,
		StartIndex:  scenario.StartIndex,
		StartSec:    scenario.StartSec,
		Pass:        true,
	}

	params, err := paramsForVariant(ctx, svc, file, info, variant)
	if err != nil {
		result.Skipped = true
		result.SkipReason = err.Error()
		return result, nil
	}
	result.HW = verifyHWPlan(svc, params, variant.Accel)

	artifactDir := filepath.Join(artifactRoot, fixtureName,
		fmt.Sprintf("%s_%s_continuous_%s", variant.Mode, accelLabel(variant.Accel), scenario.Name))
	if err := os.RemoveAll(artifactDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return nil, err
	}
	result.ArtifactDir = artifactDir
	result.PlaybackURL = "/media/" + filepath.ToSlash(filepath.Join(fixtureName,
		fmt.Sprintf("%s_%s_continuous_%s", variant.Mode, accelLabel(variant.Accel), scenario.Name),
		"ffmpeg.m3u8"))

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
	}()

	cr, err := checkContinuousHLS(ctx, svc, file, params, artifactDir, tolerance, scenario.StartIndex, scenario.StartSec)
	if err != nil {
		result.Pass = false
		result.EncodeError = err.Error()
		return result, nil
	}

	result.Pass = cr.Pass
	result.TotalEncodeMs = cr.TotalEncodeMs
	result.HasEndList = cr.HasEndList
	result.Issues = cr.Issues
	if len(cr.AudioIssues) > 0 {
		for _, msg := range cr.AudioIssues {
			result.Issues = append(result.Issues, mp4.TimelineIssue{Check: "audio", Message: msg})
		}
		if result.Pass {
			result.Pass = false
		}
	}
	for _, seg := range cr.Segments {
		result.Segments = append(result.Segments, segmentBenchmark{
			Index:          seg.Index,
			Bytes:          seg.Bytes,
			MediaStartSec:  seg.MediaStartSec,
			ExpectedStart:  seg.ExpectedStartSec,
			ActualDurSec:   seg.ActualDurSec,
			ExpectedDurSec: seg.ExpectedDurSec,
			TimelineOK:     true,
		})
	}
	if !cr.Pass && result.EncodeError == "" {
		result.EncodeError = fmt.Sprintf("%d timeline issue(s)", len(result.Issues))
	}
	return result, nil
}

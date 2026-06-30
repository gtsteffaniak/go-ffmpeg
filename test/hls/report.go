package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

type hardwareSummary struct {
	FFmpegVersion string                                                `json:"ffmpegVersion"`
	GeneratedAt   time.Time                                             `json:"generatedAt"`
	EncodeOptions []capabilities.EncodeOption                           `json:"encodeOptions,omitempty"`
	DecodeOptions []capabilities.DecodeOption                           `json:"decodeOptions,omitempty"`
	Platform      capabilities.PlatformInfo                             `json:"platform,omitempty"`
	CodecMatrix   map[capabilities.VideoCodec]capabilities.CodecSupport `json:"codecMatrix,omitempty"`
}

type fullReport struct {
	GeneratedAt     time.Time         `json:"generatedAt"`
	ReferenceVideo  string            `json:"referenceVideo"`
	FixtureDuration int               `json:"fixtureDurationSec"`
	SegmentCount    int               `json:"segmentCount"`
	Fixtures        []FixtureResult   `json:"fixtures"`
	Hardware        hardwareSummary   `json:"hardware"`
	Results         []benchmarkResult `json:"results"`
	Summary         reportSummary     `json:"summary"`
}

type reportSummary struct {
	TotalTests   int `json:"totalTests"`
	Passed       int `json:"passed"`
	Failed       int `json:"failed"`
	Skipped      int `json:"skipped"`
	FixturesOK   int `json:"fixturesGenerated"`
	FixturesFail int `json:"fixturesFailed"`
}

func buildHardwareSummary(svc *goffmpeg.Service) hardwareSummary {
	caps := cachedCapabilities(svc)
	if caps == nil {
		return hardwareSummary{GeneratedAt: time.Now()}
	}
	return hardwareSummary{
		FFmpegVersion: caps.FFmpegVersion,
		GeneratedAt:   caps.GeneratedAt,
		EncodeOptions: caps.EncodeOptions,
		DecodeOptions: caps.DecodeOptions,
		Platform:      caps.Platform,
		CodecMatrix:   caps.CodecMatrix,
	}
}

func writeReport(path string, report *fullReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func summarizeReport(report *fullReport) {
	s := reportSummary{}
	for _, f := range report.Fixtures {
		if f.Error != "" {
			s.FixturesFail++
		} else if f.Generated {
			s.FixturesOK++
		}
	}
	for _, r := range report.Results {
		s.TotalTests++
		if r.Skipped {
			s.Skipped++
		} else if r.Pass {
			s.Passed++
		} else {
			s.Failed++
		}
	}
	report.Summary = s
}

func printReportSummary(report *fullReport) {
	s := report.Summary
	fmt.Printf("\n=== Report summary ===\n")
	fmt.Printf("Fixtures: %d generated, %d failed\n", s.FixturesOK, s.FixturesFail)
	fmt.Printf("Tests: %d total, %d passed, %d failed, %d skipped\n", s.TotalTests, s.Passed, s.Failed, s.Skipped)
	fmt.Printf("Report: %s\n", filepath.Join("report_site", "data", "report.json"))
}

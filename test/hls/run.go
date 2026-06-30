package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func runFullTest(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	reference := fs.String("reference", envOr("HLS_TEST_FILE", defaultSampleVideo()), "reference video to derive fixture samples")
	fixturesDir := fs.String("fixtures", ".fixtures", "generated source files")
	reportDir := fs.String("report", "report_site", "report static site root")
	segments := fs.Int("segments", 0, "HLS segments per test (0 = full fixture timeline, ~duration/4s)")
	duration := fs.Int("duration", defaultFixtureDurationSec, "fixture sample length in seconds")
	skipGenerate := fs.Bool("skip-generate", false, "skip fixture generation (use existing files)")
	fixtureNames := fs.String("fixture-names", envOr("HLS_FIXTURE_NAMES", ""), "comma-separated fixture names (default: all)")
	softwareOnly := fs.Bool("software-only", false, "run remux/copy/transcode/software only (also when GOFFMPEG_SKIP_HW=1)")
	serve := fs.Bool("serve", false, "start report HTTP server after run")
	port := fs.Int("port", 8765, "report server port")
	debug := fs.Bool("debug", false, "ffmpeg stderr")
	tolerance := fs.Float64("tolerance", 0.12, "timeline tolerance seconds")
	_ = fs.Parse(args)

	specs := resolveFixtureSpecs(*fixtureNames)
	softwareOnlyRun := softwareOnlyRequested(*softwareOnly)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
	defer cancel()

	svc, err := initFFmpeg(ctx, *debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg init: %v\n", err)
		return 1
	}
	fmt.Println("Hardware capability detection complete (cached for all tests).")

	report := &fullReport{
		GeneratedAt:     time.Now(),
		ReferenceVideo:  *reference,
		FixtureDuration: *duration,
		SegmentCount:    *segments,
		Hardware:        buildHardwareSummary(svc),
	}

	mediaDir := filepath.Join(*reportDir, "media")
	dataDir := filepath.Join(*reportDir, "data")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "media dir: %v\n", err)
		return 1
	}

	if !*skipGenerate {
		fmt.Printf("Generating %d fixture files (%ds each) from %s …\n", len(specs), *duration, *reference)
		fixtures, err := generateFixtures(ctx, *reference, *fixturesDir, *duration, specs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate fixtures: %v\n", err)
			return 1
		}
		report.Fixtures = fixtures
		for _, f := range fixtures {
			if f.Error != "" {
				fmt.Printf("  FAIL generate %-28s %s\n", f.Spec.Name, f.Error)
			} else if f.Skipped {
				fmt.Printf("  SKIP generate %-28s (%s)\n", f.Spec.Name, f.Message)
			} else {
				fmt.Printf("  OK   generate %-28s (%dms)\n", f.Spec.Name, f.GenerateMs)
			}
		}
	} else {
		for _, spec := range specs {
			p := filepath.Join(*fixturesDir, fixtureFilename(spec))
			report.Fixtures = append(report.Fixtures, FixtureResult{Spec: spec, Path: p, Generated: true, Skipped: true, Message: "skip-generate"})
		}
	}

	caps := cachedCapabilities(svc)
	segCount := resolveSegmentCount(*segments, *duration)
	report.SegmentCount = segCount
	if softwareOnlyRun {
		fmt.Println("Running software-only benchmarks (remux, copy, transcode/software)")
	}
	fmt.Printf("\nRunning HLS benchmarks (segments=%d, fixture=%ds) …\n", segCount, *duration)

	for _, spec := range specs {
		path := filepath.Join(*fixturesDir, fixtureFilename(spec))
		info, skipReason, err := probeFixtureOrSkip(ctx, svc, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "probe %s: %v\n", spec.Name, err)
			return 1
		}
		if skipReason != "" {
			fmt.Printf("SKIP fixture %-28s (%s)\n", spec.Name, skipReason)
			continue
		}

		variants := filterSoftwareVariants(variantsForFixture(info, caps), softwareOnlyRun)
		for _, variant := range variants {
			label := fmt.Sprintf("%s/%s", spec.Name, variant.Label)
			br, err := runBenchmark(ctx, svc, path, spec.Name, variant, segCount, *tolerance, mediaDir)
			if err != nil {
				fmt.Printf("FAIL %-40s %v\n", label, err)
				report.Results = append(report.Results, benchmarkResult{
					Fixture: spec.Name, Label: variant.Label, Pass: false, EncodeError: err.Error(),
				})
				continue
			}
			report.Results = append(report.Results, *br)
			status := "PASS"
			if br.Skipped {
				status = "SKIP"
			} else if !br.Pass {
				status = "FAIL"
			}
			avgMs := int64(0)
			if len(br.Segments) > 0 {
				var sum int64
				for _, s := range br.Segments {
					sum += s.EncodeMs
				}
				avgMs = sum / int64(len(br.Segments))
			}
			fmt.Printf("%-4s %-40s encode=%dms avgSeg=%dms",
				status, label, br.TotalEncodeMs, avgMs)
			if br.Timing.WarmAvgSegMs > 0 {
				fmt.Printf(" cold=%dms warm=%dms", br.Timing.ColdSegMs, br.Timing.WarmAvgSegMs)
			}
			fmt.Printf(" cpu=%.0f%%", br.Resources.CPUPercentAvg)
			if br.Resources.GPUPercentAvg != nil {
				fmt.Printf(" gpu=%.0f%%", *br.Resources.GPUPercentAvg)
			}
			if br.EncodeError != "" {
				fmt.Printf(" err=%s", truncate(br.EncodeError, 60))
			}
			fmt.Println()
		}
	}

	summarizeReport(report)
	reportPath := filepath.Join(dataDir, "report.json")
	if err := writeReport(reportPath, report); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		return 1
	}
	printReportSummary(report)

	if report.Summary.Failed > 0 {
		if *serve {
			fmt.Println("Failures detected; serving report anyway.")
		} else {
			return 1
		}
	}

	if *serve {
		addr := fmt.Sprintf(":%d", *port)
		url := fmt.Sprintf("http://127.0.0.1:%d/", *port)
		fmt.Printf("\nReport site: %s\n", url)
		mux := reportHandler(*reportDir)
		return runHTTPServer(addr, mux)
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func reportHandler(reportDir string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(reportDir)))
	return mux
}

func runReportServe(args []string) int {
	fs := flag.NewFlagSet("serve-report", flag.ExitOnError)
	reportDir := fs.String("report", "report_site", "report static site root")
	port := fs.Int("port", 8765, "HTTP port")
	_ = fs.Parse(args)

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("Report site: http://0.0.0.0:%d/\n", *port)
	return runHTTPServer(addr, reportHandler(*reportDir))
}

// resolveSegmentCount returns segments to encode. 0 means the full HLS grid for fixture duration.
func resolveSegmentCount(segments, fixtureDurationSec int) int {
	if segments > 0 {
		return segments
	}
	if fixtureDurationSec <= 0 {
		fixtureDurationSec = defaultFixtureDurationSec
	}
	n := int(math.Ceil(float64(fixtureDurationSec) / goffmpeg.DefaultHLSSegmentDurationSec))
	if n < 1 {
		return 1
	}
	return n
}

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func runMatrix(args []string) int {
	fs := flag.NewFlagSet("matrix", flag.ExitOnError)
	reference := fs.String("reference", envOr("HLS_TEST_FILE", defaultSampleVideo()), "reference video for fixture generation")
	segments := fs.Int("segments", 3, "segments per case")
	debug := fs.Bool("debug", false, "ffmpeg stderr to terminal")
	fixtureDir := fs.String("fixtures", ".fixtures", "directory for generated fixtures")
	modes := fs.String("modes", "remux,copy,transcode", "comma-separated modes (legacy shortcut)")
	duration := fs.Int("duration", defaultFixtureDurationSec, "fixture length when generating")
	skipGenerate := fs.Bool("skip-generate", false, "use existing fixtures")
	fixtureNames := fs.String("fixture-names", envOr("HLS_FIXTURE_NAMES", ""), "comma-separated fixture names (default: all)")
	softwareOnly := fs.Bool("software-only", false, "run remux/copy/transcode/software only (also when GOFFMPEG_SKIP_HW=1)")
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

	if !*skipGenerate {
		fmt.Printf("Generating fixtures from %s …\n", *reference)
		if _, err := generateFixtures(ctx, *reference, *fixtureDir, *duration, specs); err != nil {
			fmt.Fprintf(os.Stderr, "generate: %v\n", err)
			return 1
		}
	}

	caps := cachedCapabilities(svc)
	modeList := splitCSV(*modes)
	if softwareOnlyRun {
		fmt.Printf("matrix: software-only (remux, copy, transcode/software)\n")
	}
	fmt.Printf("matrix: %d fixtures x modes [%s] (segments=%d, HW detect once)\n",
		len(specs), *modes, *segments)
	failures := 0

	for _, spec := range specs {
		path := filepath.Join(*fixtureDir, fixtureFilename(spec))
		info, skipReason, err := probeFixtureOrSkip(ctx, svc, path)
		if err != nil {
			fmt.Printf("FAIL %-24s probe: %v\n", spec.Name, err)
			failures++
			continue
		}
		if skipReason != "" {
			fmt.Printf("SKIP %-24s (%s)\n", spec.Name, skipReason)
			continue
		}

		variants := filterSoftwareVariants(variantsForFixture(info, caps), softwareOnlyRun)
		// Legacy -modes filter: only run variants whose Mode is listed
		for _, variant := range variants {
			if !modeAllowed(variant.Mode, modeList) {
				continue
			}
			label := fmt.Sprintf("%s/%s", spec.Name, variant.Label)
			br, err := runBenchmark(ctx, svc, path, spec.Name, variant, *segments, 0.05, os.TempDir())
			if err != nil {
				fmt.Printf("FAIL %s: %v\n", label, err)
				failures++
				continue
			}
			if br.Skipped {
				fmt.Printf("SKIP %s (%s)\n", label, br.SkipReason)
				continue
			}
			if !br.Pass {
				fmt.Printf("FAIL %s (%s)\n", label, br.EncodeError)
				failures++
			} else {
				fmt.Printf("PASS %s\n", label)
			}
		}
	}

	if failures > 0 {
		fmt.Printf("matrix: %d failure(s)\n", failures)
		return 1
	}
	fmt.Println("matrix: all passed")
	return 0
}

func modeAllowed(mode string, allowed []string) bool {
	for _, m := range allowed {
		if m == mode {
			return true
		}
	}
	return false
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range splitOnComma(s) {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func splitOnComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, trimSpace(s[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, trimSpace(s[start:]))
	return parts
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

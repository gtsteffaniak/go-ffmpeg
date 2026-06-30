package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func runGenerateFixtures(args []string) int {
	fs := flag.NewFlagSet("generate-fixtures", flag.ExitOnError)
	reference := fs.String("reference", envOr("HLS_TEST_FILE", defaultSampleVideo()), "reference video")
	outDir := fs.String("out", ".fixtures", "output directory")
	duration := fs.Int("duration", defaultFixtureDurationSec, "sample length in seconds")
	fixtureNames := fs.String("fixture-names", envOr("HLS_FIXTURE_NAMES", ""), "comma-separated fixture names (default: all)")
	_ = fs.Parse(args)

	specs := resolveFixtureSpecs(*fixtureNames)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	fmt.Printf("Generating %d fixtures (%ds) from %s → %s\n", len(specs), *duration, *reference, *outDir)
	results, err := generateFixtures(ctx, *reference, *outDir, *duration, specs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		return 1
	}
	fail := 0
	for _, f := range results {
		if f.Error != "" {
			fail++
			fmt.Printf("FAIL %-28s %s\n", f.Spec.Name, f.Error)
		} else if f.Skipped {
			fmt.Printf("SKIP %-28s (%s)\n", f.Spec.Name, f.Message)
		} else {
			fmt.Printf("OK   %-28s %dms\n", f.Spec.Name, f.GenerateMs)
		}
	}
	if fail > 0 {
		return 1
	}
	return 0
}

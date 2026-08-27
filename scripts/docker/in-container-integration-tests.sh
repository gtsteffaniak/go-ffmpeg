#!/usr/bin/env bash
# Full integration suite inside go-ffmpeg-test container.
set -euo pipefail

VERSION="${1:-unknown}"
echo "==> integration tests (ffmpeg ${VERSION})"
ffmpeg -version | head -1

sample="${GOFFMPEG_SAMPLE_MP4:-test/data/Big_Buck_Bunny_1080_10s_2MB.mp4}"
test -f "$sample"

go test -tags=integration ./... -run "Integration|ContinuousSoftware" -count=1

go build -o bin/go-ffmpeg ./cmd/go-ffmpeg
./bin/go-ffmpeg -json | head -20
./bin/go-ffmpeg -skip-hw-tests | head -15

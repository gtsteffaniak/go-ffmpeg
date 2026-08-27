#!/usr/bin/env bash
# Full integration suite inside go-ffmpeg-test container.
set -euo pipefail

VERSION="${1:-unknown}"
echo "==> integration tests (ffmpeg ${VERSION})"
ffmpeg -version 2>&1 | sed -n '1p'

sample="${GOFFMPEG_SAMPLE_MP4:-test/data/Big_Buck_Bunny_1080_10s_2MB.mp4}"
test -f "$sample"

go test -tags=integration ./... -run "Integration|ContinuousSoftware" -count=1

cli="$(mktemp /tmp/go-ffmpeg-XXXXXX)"
go build -o "$cli" ./cmd/go-ffmpeg
"$cli" -json | sed -n '1,20p'
"$cli" -skip-hw-tests | sed -n '1,15p'
rm -f "$cli"

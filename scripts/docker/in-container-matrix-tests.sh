#!/usr/bin/env bash
# Runs inside go-ffmpeg-test:* container (see docker/test.Dockerfile).
set -euo pipefail

VERSION="${1:-unknown}"
echo "==> ffmpeg matrix tests (ffmpeg ${VERSION})"
ffmpeg -version 2>&1 | sed -n '1p'

echo "==> version-gated unit tests"
go test ./capabilities ./encode -count=1 \
	-run 'TestDetect|TestFeatureFlags|TestAppendReadrate|TestAppendReadrateArgsLivePaced'

sample="${GOFFMPEG_SAMPLE_MP4:-test/data/Big_Buck_Bunny_1080_10s_2MB.mp4}"
if [[ -f "$sample" ]]; then
	echo "==> integration subset"
	go test -tags=integration ./... \
		-run 'TestIntegration(Detect|MediaDuration|Screenshot)' -count=1
else
	echo "skip integration subset: missing $sample" >&2
fi

echo "==> CLI compatibility report"
cli="$(mktemp /tmp/go-ffmpeg-XXXXXX)"
go build -o "$cli" ./cmd/go-ffmpeg
"$cli" -skip-hw-tests -json | sed -n '1,10p'
rm -f "$cli"

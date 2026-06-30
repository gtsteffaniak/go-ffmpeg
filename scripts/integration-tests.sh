#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SAMPLE="$ROOT/test/data/Big_Buck_Bunny_1080_10s_2MB.mp4"
SEGMENTS="${SEGMENTS:-3}"

if [[ ! -f "$SAMPLE" ]]; then
  echo "missing sample video: $SAMPLE" >&2
  exit 1
fi

echo "=== [1/3] Library integration tests ==="
GOFFMPEG_SAMPLE_MP4="$SAMPLE" GOFFMPEG_SKIP_HW="${GOFFMPEG_SKIP_HW:-1}" \
  go test -tags=integration ./... -run Integration -v

echo ""
echo "=== [2/3] HLS harness unit tests ==="
(cd test/hls && go test -count=1 ./...)

echo ""
echo "=== [3/3] HLS benchmark matrix → test/hls/report_site/data/report.json ==="
(cd test/hls && make generate-results SEGMENTS="$SEGMENTS")

if [[ ! -f test/hls/report_site/data/report.json ]]; then
  echo "expected report missing: test/hls/report_site/data/report.json" >&2
  exit 1
fi

echo ""
echo "Integration tests complete."
echo "View dashboard: make serve-results"

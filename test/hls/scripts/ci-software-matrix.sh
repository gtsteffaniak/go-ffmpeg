#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SAMPLE="${SAMPLE:-$ROOT/../data/Big_Buck_Bunny_1080_10s_2MB.mp4}"
REFERENCE="${REFERENCE:-$SAMPLE}"
FIXTURES="${FIXTURES:-$ROOT/.fixtures}"
DURATION="${DURATION:-10}"
SEGMENTS="${SEGMENTS:-3}"
FIXTURE_NAMES="${HLS_FIXTURE_NAMES:-h264_aac_mp4,h264_aac_mkv,h264_aac_mov,h264_mp3_mkv,h264_ac3_mkv,h264_eac3_mkv,h264_aac_avi,h264_mp3_avi}"

if [[ ! -f "$REFERENCE" ]]; then
  echo "missing default sample video: $REFERENCE" >&2
  exit 1
fi

echo "Using reference sample: $REFERENCE"

echo "Building HLS test harness …"
go build -o test-hls .

export GOFFMPEG_SKIP_HW=1
export HLS_SOFTWARE_ONLY=1
export HLS_FIXTURE_NAMES="$FIXTURE_NAMES"

echo "Generating fixtures (${FIXTURE_NAMES}) …"
./test-hls generate-fixtures \
  -reference "$REFERENCE" \
  -out "$FIXTURES" \
  -duration "$DURATION" \
  -fixture-names "$FIXTURE_NAMES"

echo "Running software-only HLS matrix (segments=${SEGMENTS}) …"
./test-hls matrix \
  -reference "$REFERENCE" \
  -fixtures "$FIXTURES" \
  -skip-generate \
  -segments "$SEGMENTS" \
  -duration "$DURATION" \
  -fixture-names "$FIXTURE_NAMES" \
  -software-only

echo "HLS software matrix passed."

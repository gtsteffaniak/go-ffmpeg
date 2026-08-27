#!/usr/bin/env bash
# Pre-push: parallel ffmpeg matrix in docker (matches CI Jenkins gates).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$REPO_ROOT"

sample="${GOFFMPEG_SAMPLE_MP4:-test/data/Big_Buck_Bunny_1080_10s_2MB.mp4}"
if [[ ! -f "$sample" ]]; then
	echo "missing sample video: $sample" >&2
	echo "Add test/data/Big_Buck_Bunny_1080_10s_2MB.mp4 or set GOFFMPEG_SAMPLE_MP4." >&2
	exit 1
fi

bash "$REPO_ROOT/scripts/docker/run-matrix-parallel.sh"

#!/usr/bin/env bash
# Shared docker run flags for in-container tests (parallel-safe).
set -euo pipefail

docker_test_run() {
	local image="$1"
	shift
	local repo_root
	repo_root="$(cd "$(dirname "$0")/../.." && pwd)"

	docker run --rm \
		-v "$repo_root:/src:ro" \
		-w /src \
		-e GOFFMPEG_SKIP_HW=1 \
		-e GOFFMPEG_FFMPEG_PATH=/usr/local/bin/ffmpeg \
		-e GOFFMPEG_FFPROBE_PATH=/usr/local/bin/ffprobe \
		-e GOMODCACHE=/gomodcache \
		-e GOCACHE=/tmp/gocache \
		-e GOFLAGS=-buildvcs=false \
		-e GOFFMPEG_SAMPLE_MP4=test/data/Big_Buck_Bunny_1080_10s_2MB.mp4 \
		"$image" \
		"$@"
}

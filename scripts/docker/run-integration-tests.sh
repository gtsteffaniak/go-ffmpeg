#!/usr/bin/env bash
# Integration suite in container (default ffmpeg 8.1.2).
set -euo pipefail

VERSION="${GOFFMPEG_MATRIX_VERSION:-8.1.2}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/docker/in-container-integration-tests.sh"

TEST_IMAGE="$(bash "$REPO_ROOT/scripts/docker/build-test-image.sh" "$VERSION")"
if [[ -z "$TEST_IMAGE" ]]; then
	exit 1
fi

docker run --rm \
	-v "$REPO_ROOT:/src" \
	-w /src \
	-e GOFFMPEG_SKIP_HW=1 \
	-e GOFFMPEG_FFMPEG_PATH=/usr/local/bin/ffmpeg \
	-e GOFFMPEG_FFPROBE_PATH=/usr/local/bin/ffprobe \
	-e GOFFMPEG_SAMPLE_MP4=test/data/Big_Buck_Bunny_1080_10s_2MB.mp4 \
	"$TEST_IMAGE" \
	bash "$SCRIPT" "$VERSION"

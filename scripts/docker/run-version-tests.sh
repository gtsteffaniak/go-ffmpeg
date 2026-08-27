#!/usr/bin/env bash
# Build go-ffmpeg-test image for one ffmpeg version and run matrix gate tests.
set -euo pipefail

VERSION="${1:?ffmpeg version required, e.g. 8.1.2}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/docker/in-container-matrix-tests.sh"

TEST_IMAGE="$(bash "$REPO_ROOT/scripts/docker/build-test-image.sh" "$VERSION")"
if [[ -z "$TEST_IMAGE" ]]; then
	exit 0
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

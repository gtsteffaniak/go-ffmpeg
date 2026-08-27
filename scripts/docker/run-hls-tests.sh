#!/usr/bin/env bash
# HLS software matrix inside container (default ffmpeg 8.1.2).
set -euo pipefail

VERSION="${GOFFMPEG_MATRIX_VERSION:-8.1.2}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

TEST_IMAGE="$(bash "$REPO_ROOT/scripts/docker/build-test-image.sh" "$VERSION" | tail -n1)"
if [[ -z "$TEST_IMAGE" ]]; then
	exit 1
fi

docker run --rm \
	-v "$REPO_ROOT:/src" \
	-w /src \
	-e GOFFMPEG_SKIP_HW=1 \
	-e GOFFMPEG_FFMPEG_PATH=/usr/local/bin/ffmpeg \
	-e GOFFMPEG_FFPROBE_PATH=/usr/local/bin/ffprobe \
	-e HLS_SOFTWARE_ONLY=1 \
	"$TEST_IMAGE" \
	bash -c 'make test-hls SEGMENTS=0 FIXTURE_DURATION=32 \
		FIXTURE_NAMES=h264_aac_mp4,h264_aac_mkv,h264_eac3_mkv,wmv2_wmapro_wmv'

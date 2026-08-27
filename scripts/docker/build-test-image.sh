#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?ffmpeg version required, e.g. 8.1.2}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FFMPEG_IMAGE="${GOFFMPEG_IMAGE:-gtstef/ffmpeg:${VERSION}}"
TEST_IMAGE="${GOFFMPEG_TEST_IMAGE:-go-ffmpeg-test:${VERSION}}"

cd "$REPO_ROOT"

if ! docker image inspect "$FFMPEG_IMAGE" &>/dev/null; then
	if ! docker pull "$FFMPEG_IMAGE"; then
		if [[ "${GOFFMPEG_MATRIX_SKIP_MISSING:-}" == "1" ]]; then
			echo "skip: $FFMPEG_IMAGE not available (GOFFMPEG_MATRIX_SKIP_MISSING=1)"
			exit 0
		fi
		echo "failed to pull $FFMPEG_IMAGE" >&2
		exit 1
	fi
fi

docker build -f docker/test.Dockerfile \
	--build-arg "FFMPEG_IMAGE=${FFMPEG_IMAGE}" \
	-t "$TEST_IMAGE" \
	. >&2

echo "$TEST_IMAGE"

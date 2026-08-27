#!/usr/bin/env bash
# Build go-ffmpeg-test image for one ffmpeg version and run matrix gate tests.
set -euo pipefail

VERSION="${1:?ffmpeg version required, e.g. 8.1.2}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CONTAINER_SCRIPT="/src/scripts/docker/in-container-matrix-tests.sh"
DOCKER_RUN="$REPO_ROOT/scripts/docker/docker-test-run.sh"

TEST_IMAGE="${GOFFMPEG_TEST_IMAGE:-go-ffmpeg-test:${VERSION}}"

if [[ "${GOFFMPEG_SKIP_IMAGE_BUILD:-}" != "1" ]]; then
	TEST_IMAGE="$(bash "$REPO_ROOT/scripts/docker/build-test-image.sh" "$VERSION" | tail -n1)"
	if [[ -z "$TEST_IMAGE" ]]; then
		exit 0
	fi
elif ! docker image inspect "$TEST_IMAGE" &>/dev/null; then
	echo "missing test image $TEST_IMAGE (build first)" >&2
	exit 1
fi

# shellcheck source=/dev/null
source "$DOCKER_RUN"
docker_test_run "$TEST_IMAGE" bash "$CONTAINER_SCRIPT" "$VERSION"

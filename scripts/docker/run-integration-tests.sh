#!/usr/bin/env bash
# Integration suite in container (default ffmpeg 8.1.2).
set -euo pipefail

VERSION="${GOFFMPEG_MATRIX_VERSION:-8.1.2}"
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DOCKER_RUN="$REPO_ROOT/scripts/docker/docker-test-run.sh"

TEST_IMAGE="$(bash "$REPO_ROOT/scripts/docker/build-test-image.sh" "$VERSION" | tail -n1)"
if [[ -z "$TEST_IMAGE" ]]; then
	exit 1
fi

# shellcheck source=/dev/null
source "$DOCKER_RUN"
docker_test_run "$TEST_IMAGE" bash /src/scripts/docker/in-container-integration-tests.sh "$VERSION"

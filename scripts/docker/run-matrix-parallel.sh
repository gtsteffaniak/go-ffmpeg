#!/usr/bin/env bash
# Run matrix gate tests for every version in docker/versions (parallel).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VERSIONS_FILE="${GOFFMPEG_VERSIONS_FILE:-$REPO_ROOT/docker/versions}"
LOG_DIR="${GOFFMPEG_MATRIX_LOG_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/go-ffmpeg-matrix-logs.XXXXXX")}"

if [[ ! -f "$VERSIONS_FILE" ]]; then
	echo "missing versions file: $VERSIONS_FILE" >&2
	exit 1
fi

if ! command -v docker &>/dev/null; then
	echo "docker is required for ffmpeg matrix tests" >&2
	exit 1
fi

versions=()
while IFS= read -r line || [[ -n "$line" ]]; do
	line="${line//$'\r'/}"
	[[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
	versions+=("$line")
done < "$VERSIONS_FILE"
if [[ ${#versions[@]} -eq 0 ]]; then
	echo "no versions in $VERSIONS_FILE" >&2
	exit 1
fi

echo "ffmpeg matrix (${#versions[@]} versions): ${versions[*]}"
echo "matrix logs: $LOG_DIR"

echo "==> building test images"
for version in "${versions[@]}"; do
	echo "  build $version"
	if ! bash "$REPO_ROOT/scripts/docker/build-test-image.sh" "$version" >"$LOG_DIR/build-$version.log" 2>&1; then
		echo "FAILED: build ffmpeg $version" >&2
		cat "$LOG_DIR/build-$version.log" >&2
		exit 1
	fi
done

echo "==> running tests in parallel"
pids=()
names=()
for version in "${versions[@]}"; do
	(
		GOFFMPEG_SKIP_IMAGE_BUILD=1 \
			bash "$REPO_ROOT/scripts/docker/run-version-tests.sh" "$version"
	) >"$LOG_DIR/test-$version.log" 2>&1 &
	pids+=($!)
	names+=("$version")
done

failed=0
for i in "${!pids[@]}"; do
	version="${names[$i]}"
	if ! wait "${pids[$i]}"; then
		echo "FAILED: ffmpeg $version (see $LOG_DIR/test-$version.log)" >&2
		cat "$LOG_DIR/test-$version.log" >&2
		failed=1
	else
		echo "ok: ffmpeg $version"
	fi
done

if [[ "$failed" -ne 0 ]]; then
	exit 1
fi

echo "ffmpeg matrix: all versions passed"

#!/usr/bin/env bash
# Run matrix gate tests for every version in docker/versions (parallel).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VERSIONS_FILE="${GOFFMPEG_VERSIONS_FILE:-$REPO_ROOT/docker/versions}"

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

echo "ffmpeg matrix (${#versions[@]} versions, parallel): ${versions[*]}"

pids=()
names=()
for version in "${versions[@]}"; do
	(
		echo "--- start $version ---"
		bash "$REPO_ROOT/scripts/docker/run-version-tests.sh" "$version"
		echo "--- ok $version ---"
	) &
	pids+=($!)
	names+=("$version")
done

failed=0
for i in "${!pids[@]}"; do
	if ! wait "${pids[$i]}"; then
		echo "FAILED: ffmpeg ${names[$i]}" >&2
		failed=1
	fi
done

if [[ "$failed" -ne 0 ]]; then
	exit 1
fi

echo "ffmpeg matrix: all versions passed"

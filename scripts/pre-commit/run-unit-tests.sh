#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

if [[ -z "${GOFFMPEG_FFMPEG_PATH:-}" ]]; then
  if ! command -v ffmpeg &>/dev/null; then
    echo "ffmpeg is required for unit tests." >&2
    echo "Install ffmpeg locally, or set GOFFMPEG_FFMPEG_PATH and GOFFMPEG_FFPROBE_PATH." >&2
    echo "See CONTRIBUTING.md (Pre-commit hooks)." >&2
    exit 1
  fi
fi

if [[ -z "${GOFFMPEG_FFPROBE_PATH:-}" ]] && ! command -v ffprobe &>/dev/null; then
  echo "ffprobe is required for unit tests." >&2
  echo "Install ffmpeg locally, or set GOFFMPEG_FFPROBE_PATH." >&2
  exit 1
fi

go test ./... -count=1 -short

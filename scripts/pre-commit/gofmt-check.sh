#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

unformatted="$(gofmt -s -l . test/hls)"
if [[ -n "$unformatted" ]]; then
  echo "gofmt -s found unformatted files:" >&2
  echo "$unformatted" >&2
  echo "Run: make format" >&2
  exit 1
fi

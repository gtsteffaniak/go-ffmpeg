#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

go vet ./...
go vet -C test/hls ./...

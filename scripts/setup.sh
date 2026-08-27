#!/usr/bin/env bash
# One-time (or repeat-safe) dev environment setup: pre-commit hooks + requirement checks.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

ensure_pre_commit() {
	if command -v pre-commit &>/dev/null; then
		return 0
	fi
	echo "pre-commit not found; installing..."
	if command -v brew &>/dev/null; then
		brew install pre-commit
	elif command -v pip3 &>/dev/null; then
		pip3 install --user pre-commit
	elif command -v pip &>/dev/null; then
		pip install --user pre-commit
	else
		echo "Install pre-commit manually, then re-run make setup:" >&2
		echo "  brew install pre-commit" >&2
		echo "  pip install pre-commit" >&2
		exit 1
	fi
	if ! command -v pre-commit &>/dev/null; then
		echo "pre-commit installed but not on PATH; add pip --user bin to PATH and re-run make setup" >&2
		exit 1
	fi
}

echo "==> Installing pre-commit"
ensure_pre_commit

echo "==> Installing git hooks (pre-commit)"
pre-commit install

echo ""
echo "==> Checking dev requirements"
warn=0

check_cmd() {
	local name="$1" note="$2"
	if command -v "$name" &>/dev/null; then
		echo "  ok: $name"
	else
		echo "  missing: $name — $note" >&2
		warn=1
	fi
}

check_cmd go "required to build and test"
check_cmd ffmpeg "required for unit tests and pre-commit go-test-unit hook"
check_cmd ffprobe "required for unit tests and pre-commit go-test-unit hook"
check_cmd docker "required for pre-commit ffmpeg matrix (gtstef/ffmpeg, see docker/versions)"

sample="test/data/Big_Buck_Bunny_1080_10s_2MB.mp4"
if [[ -f "$sample" ]]; then
	echo "  ok: $sample"
else
	echo "  missing: $sample — required for ffmpeg matrix hook" >&2
	warn=1
fi

echo ""
if [[ "$warn" -eq 1 ]]; then
	echo "Setup complete (hooks installed). Install missing tools above before committing."
else
	echo "Setup complete. Hooks run on commit: lint, unit tests, and ffmpeg matrix in docker."
fi

#!/usr/bin/env bash
# Replaces pre-commit-hooks trailing-whitespace (no Python/pip required).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

failed=0
while IFS= read -r -d '' file; do
	[[ -f "$file" ]] || continue
	if grep -n '[[:blank:]]$' "$file" >/dev/null 2>&1; then
		echo "trailing whitespace: $file" >&2
		failed=1
	fi
done < <(git diff --cached --name-only -z --diff-filter=ACM)

exit "$failed"

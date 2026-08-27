#!/usr/bin/env bash
# Replaces pre-commit-hooks check-merge-conflict (no Python/pip required).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

failed=0
while IFS= read -r -d '' file; do
	[[ -f "$file" ]] || continue
	if grep -qE '^(<<<<<<<|=======|>>>>>>>)' "$file" 2>/dev/null; then
		echo "merge conflict marker in: $file" >&2
		failed=1
	fi
done < <(git diff --cached --name-only -z --diff-filter=ACM)

exit "$failed"

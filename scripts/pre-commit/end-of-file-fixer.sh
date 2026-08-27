#!/usr/bin/env bash
# Ensures staged text files end with a newline (no Python/pip required).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

fixed=0
while IFS= read -r -d '' file; do
	[[ -f "$file" ]] || continue
	# Skip empty files and binary-ish paths.
	case "$file" in
		*.png|*.jpg|*.jpeg|*.gif|*.webp|*.mp4|*.mkv|*.zip|*.xz|*.gz) continue ;;
	esac
	if [[ ! -s "$file" ]]; then
		continue
	fi
	if [[ $(tail -c1 "$file" | wc -l | tr -d ' ') -eq 0 ]]; then
		printf '\n' >>"$file"
		git add "$file"
		echo "fixed missing EOF newline: $file"
		fixed=1
	fi
done < <(git diff --cached --name-only -z --diff-filter=ACM)

if [[ "$fixed" -eq 1 ]]; then
	echo "Re-staged files with EOF newlines; review and commit again if needed."
	exit 1
fi

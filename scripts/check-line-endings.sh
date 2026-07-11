#!/usr/bin/env bash
set -euo pipefail

files="$(mktemp)"
errors="$(mktemp)"
cleanup() {
  rm -f "$files" "$errors"
}
trap cleanup EXIT

if git ls-files -z -- '*.sh' >"$files" 2>"$errors"; then
  :
elif command -v git.exe >/dev/null 2>&1 && git.exe ls-files -z -- '*.sh' >"$files" 2>"$errors"; then
  :
else
  echo "unable to list tracked shell scripts" >&2
  exit 1
fi

failed=0
while IFS= read -r -d '' file; do
  if LC_ALL=C grep -q $'\r' "$file"; then
    echo "$file: shell script must use LF line endings" >&2
    failed=1
  fi
done <"$files"

if [ "$failed" -ne 0 ]; then
  exit 1
fi

echo "line-endings-ok"

#!/usr/bin/env bash
set -euo pipefail

failed=0
while IFS= read -r -d '' file; do
  if LC_ALL=C grep -q $'\r' "$file"; then
    echo "$file: shell script must use LF line endings" >&2
    failed=1
  fi
done < <(git ls-files -z -- '*.sh')

if [ "$failed" -ne 0 ]; then
  exit 1
fi

echo "line-endings-ok"

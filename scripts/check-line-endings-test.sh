#!/usr/bin/env bash
set -euo pipefail

root="$(pwd)"
checker="$root/scripts/check-line-endings.sh"
tmpdir="$(mktemp -d)"

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

git -C "$tmpdir" init -q

printf '#!/usr/bin/env bash\necho ok\n' >"$tmpdir/good.sh"
git -C "$tmpdir" add good.sh

good_output="$(cd "$tmpdir" && bash "$checker")"
grep -q '^line-endings-ok$' <<<"$good_output"

printf '#!/usr/bin/env bash\r\necho bad\r\n' >"$tmpdir/bad.sh"
git -C "$tmpdir" add bad.sh

if bad_output="$(cd "$tmpdir" && bash "$checker" 2>&1)"; then
  echo "check-line-endings unexpectedly accepted CRLF" >&2
  exit 1
fi
grep -q '^bad.sh: shell script must use LF line endings$' <<<"$bad_output"

echo "line-endings-test-ok"

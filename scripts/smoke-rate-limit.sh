#!/usr/bin/env bash
set -euo pipefail

addr="${GATEWAY_RATE_LIMIT_ADDR:-127.0.0.1:18082}"
api_key="${GATEWAY_RATE_LIMIT_API_KEY:-rate-limit-smoke-key}"
tmpdir="$(mktemp -d)"
config="$tmpdir/config.json"
headers="$tmpdir/headers.txt"
body="$tmpdir/body.json"
log="${TMPDIR:-/tmp}/open-ai-gateway-rate-limit-smoke.log"

cleanup() {
  if [ -n "${pid:-}" ]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf "$tmpdir"
}
trap cleanup EXIT

cat >"$config" <<JSON
{
  "addr": "$addr",
  "api_key": "$api_key",
  "rate_limit": {
    "requests_per_minute": 1
  },
  "providers": {
    "fake": {
      "type": "fake"
    }
  },
  "models": {
    "test-model": {
      "provider": "fake",
      "capabilities": ["chat", "embeddings"]
    }
  }
}
JSON

GATEWAY_CONFIG="$config" \
  GATEWAY_ADDR= \
  GATEWAY_API_KEY= \
  GATEWAY_API_KEYS= \
  go run ./cmd/gateway >"$log" 2>&1 &
pid=$!

for _ in $(seq 1 30); do
  if curl -fsS "http://$addr/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

curl -fsS "http://$addr/healthz" | grep -q '"status":"ok"'

curl -fsS "http://$addr/v1/models" \
  -H "Authorization: Bearer $api_key" \
  | grep -q '"id":"test-model"'

status="$(curl -sS -D "$headers" -o "$body" -w "%{http_code}" "http://$addr/v1/models" \
  -H "Authorization: Bearer $api_key")"

test "$status" = "429"
grep -q '"type":"rate_limit_error"' "$body"
tr -d '\r' <"$headers" | grep -Eqi '^Retry-After: [1-9][0-9]*$'

metrics="$(curl -fsS "http://$addr/metrics")"
grep -q 'open_ai_gateway_rate_limit_rejections_total{path="/v1/models",client="default"} 1' <<<"$metrics"

echo "smoke-rate-limit-ok"

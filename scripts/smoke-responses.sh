#!/usr/bin/env bash
set -euo pipefail

addr="${GATEWAY_ADDR:-127.0.0.1:18083}"
api_key="${GATEWAY_API_KEY:-test-gateway-key}"
log="${TMPDIR:-/tmp}/open-ai-gateway-responses-smoke.log"

GATEWAY_ADDR="$addr" GATEWAY_API_KEY="$api_key" go run ./cmd/gateway >"$log" 2>&1 &
pid=$!

cleanup() {
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
}
trap cleanup EXIT

for _ in $(seq 1 30); do
  if curl -fsS "http://$addr/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

response="$(curl -fsS "http://$addr/v1/responses" \
  -H "Authorization: Bearer $api_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"test-model","input":"hello"}')"
grep -q '"object":"response"' <<<"$response"
grep -q '"type":"output_text"' <<<"$response"
grep -q 'Hello from open-ai-gateway' <<<"$response"

stream="$(curl -fsS -N "http://$addr/v1/responses" \
  -H "Authorization: Bearer $api_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"test-model","input":"hello","stream":true}')"
grep -q '^event: response.created$' <<<"$stream"
grep -q '^event: response.output_text.delta$' <<<"$stream"
grep -q '^event: response.completed$' <<<"$stream"
if grep -q '\[DONE\]' <<<"$stream"; then
  echo "Responses stream unexpectedly contained [DONE]" >&2
  exit 1
fi

echo "responses-smoke-ok"

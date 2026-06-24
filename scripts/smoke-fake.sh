#!/usr/bin/env bash
set -euo pipefail

addr="${GATEWAY_ADDR:-127.0.0.1:18080}"
api_key="${GATEWAY_API_KEY:-test-gateway-key}"
log="${TMPDIR:-/tmp}/open-ai-gateway-smoke.log"

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

curl -fsS "http://$addr/healthz" | grep -q '"status":"ok"'

curl -fsS "http://$addr/readyz" | grep -q '"status":"ready"'

curl -fsS "http://$addr/version" | grep -q '"version"'

curl -fsS "http://$addr/metrics" \
  | grep -q 'open_ai_gateway_http_requests_total'

curl -fsS "http://$addr/v1/models" \
  -H "Authorization: Bearer $api_key" \
  | grep -q '"id":"test-model"'

curl -fsS "http://$addr/v1/chat/completions" \
  -H "Authorization: Bearer $api_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"test-model","messages":[{"role":"user","content":"hello"}]}' \
  | grep -q '"object":"chat.completion"'

curl -fsS -N "http://$addr/v1/chat/completions" \
  -H "Authorization: Bearer $api_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"test-model","stream":true,"messages":[{"role":"user","content":"hello"}]}' \
  | grep -q 'data: \[DONE\]'

curl -fsS "http://$addr/v1/embeddings" \
  -H "Authorization: Bearer $api_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"test-model","input":"hello"}' \
  | grep -q '"object":"list"'

echo "smoke-ok"

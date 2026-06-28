#!/usr/bin/env bash
set -euo pipefail

if [ -z "${DEEPSEEK_API_KEY:-}" ]; then
  echo "smoke-deepseek-skip: DEEPSEEK_API_KEY is not set"
  exit 0
fi

addr="${GATEWAY_ADDR:-127.0.0.1:18081}"
api_key="${GATEWAY_API_KEY:-trae-local-gateway-key}"
model="${DEEPSEEK_SMOKE_MODEL:-deepseek-v4-flash}"
log="${TMPDIR:-/tmp}/open-ai-gateway-deepseek-smoke.log"

GATEWAY_CONFIG=config.deepseek.example.json \
  GATEWAY_ADDR="$addr" \
  GATEWAY_API_KEY="$api_key" \
  DEEPSEEK_API_KEY="$DEEPSEEK_API_KEY" \
  go run ./cmd/gateway >"$log" 2>&1 &
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

curl -fsS "http://$addr/v1/models" \
  -H "Authorization: Bearer $api_key" \
  | grep -q "\"id\":\"$model\""

curl -fsS "http://$addr/v1/chat/completions" \
  -H "Authorization: Bearer $api_key" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"$model\",\"messages\":[{\"role\":\"user\",\"content\":\"Reply with ok.\"}]}" \
  | grep -q '"object":"chat.completion"'

if [ "${DEEPSEEK_SMOKE_STREAM:-0}" = "1" ]; then
  curl -fsS -N "http://$addr/v1/chat/completions" \
    -H "Authorization: Bearer $api_key" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"$model\",\"stream\":true,\"messages\":[{\"role\":\"user\",\"content\":\"Reply with ok.\"}]}" \
    | grep -q 'data: \[DONE\]'
fi

echo "smoke-deepseek-ok"

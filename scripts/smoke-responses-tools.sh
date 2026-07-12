#!/usr/bin/env bash
set -euo pipefail

addr="${GATEWAY_RESPONSES_TOOLS_SMOKE_ADDR:-127.0.0.1:18085}"
api_key="${GATEWAY_API_KEY:-test-gateway-key}"
log="${TMPDIR:-/tmp}/open-ai-gateway-responses-tools-smoke.log"

GATEWAY_ADDR="$addr" GATEWAY_API_KEY="$api_key" go run ./cmd/gateway >"$log" 2>&1 &
pid=$!
cleanup() { kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; }
trap cleanup EXIT

for _ in $(seq 1 30); do
  curl -fsS "http://$addr/healthz" >/dev/null 2>&1 && break
  sleep 1
done

tools='[{"type":"function","name":"get_weather","parameters":{"type":"object","properties":{"location":{"type":"string"}},"required":["location"],"additionalProperties":false}}]'
first="$(curl -fsS "http://$addr/v1/responses" -H "Authorization: Bearer $api_key" -H 'Content-Type: application/json' -d "{\"model\":\"test-model\",\"input\":\"weather\",\"tools\":$tools}")"
grep -q '"type":"function_call"' <<<"$first"
grep -q '"call_id":"call_fake_weather"' <<<"$first"

second='{"model":"test-model","input":[{"type":"function_call","id":"fc_client","call_id":"call_fake_weather","name":"get_weather","arguments":"{\"location\":\"Paris\"}","status":"completed"},{"type":"function_call_output","call_id":"call_fake_weather","output":"25C"}],"tools":'
final="$(curl -fsS "http://$addr/v1/responses" -H "Authorization: Bearer $api_key" -H 'Content-Type: application/json' -d "$second$tools}")"
grep -q 'Tool result received: 25C' <<<"$final"

stream="$(curl -fsS -N "http://$addr/v1/responses" -H "Authorization: Bearer $api_key" -H 'Content-Type: application/json' -d "{\"model\":\"test-model\",\"input\":\"weather\",\"stream\":true,\"tools\":$tools}")"
grep -q '^event: response.function_call_arguments.delta$' <<<"$stream"
grep -q '^event: response.function_call_arguments.done$' <<<"$stream"
grep -q '^event: response.completed$' <<<"$stream"

echo "responses-tools-smoke-ok"

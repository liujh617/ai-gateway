#!/usr/bin/env bash
set -euo pipefail

addr="${GATEWAY_RESPONSES_STATE_SMOKE_ADDR:-127.0.0.1:18086}"
api_key="${GATEWAY_API_KEY:-test-gateway-key}"
log="${TMPDIR:-/tmp}/open-ai-gateway-responses-state-smoke.log"

GATEWAY_ADDR="$addr" GATEWAY_API_KEY="$api_key" go run ./cmd/gateway >"$log" 2>&1 &
pid=$!
cleanup() { kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; }
trap cleanup EXIT

for _ in $(seq 1 30); do
  curl -fsS "http://$addr/healthz" >/dev/null 2>&1 && break
  sleep 1
done

response_id() {
  grep -o '"id":"resp_[^"]*"' <<<"$1" | head -n 1 | cut -d'"' -f4
}

first="$(curl -fsS "http://$addr/v1/responses" -H "Authorization: Bearer $api_key" -H 'Content-Type: application/json' -d '{"model":"test-model","input":"hello"}')"
first_id="$(response_id "$first")"
test -n "$first_id"
second="$(curl -fsS "http://$addr/v1/responses" -H "Authorization: Bearer $api_key" -H 'Content-Type: application/json' -d "{\"model\":\"test-model\",\"input\":\"again\",\"previous_response_id\":\"$first_id\"}")"
grep -q "\"previous_response_id\":\"$first_id\"" <<<"$second"

tools='[{"type":"function","name":"get_weather","parameters":{"type":"object","properties":{"location":{"type":"string"}},"required":["location"],"additionalProperties":false}}]'
function_first="$(curl -fsS "http://$addr/v1/responses" -H "Authorization: Bearer $api_key" -H 'Content-Type: application/json' -d "{\"model\":\"test-model\",\"input\":\"weather\",\"tools\":$tools}")"
function_id="$(response_id "$function_first")"
grep -q '"call_id":"call_fake_weather"' <<<"$function_first"
function_second="$(curl -fsS "http://$addr/v1/responses" -H "Authorization: Bearer $api_key" -H 'Content-Type: application/json' -d "{\"model\":\"test-model\",\"previous_response_id\":\"$function_id\",\"input\":[{\"type\":\"function_call_output\",\"call_id\":\"call_fake_weather\",\"output\":\"25C\"}]}")"
grep -q 'Tool result received: 25C' <<<"$function_second"

unstored="$(curl -fsS "http://$addr/v1/responses" -H "Authorization: Bearer $api_key" -H 'Content-Type: application/json' -d '{"model":"test-model","input":"temporary","store":false}')"
unstored_id="$(response_id "$unstored")"
status="$(curl -sS -o /dev/null -w '%{http_code}' "http://$addr/v1/responses" -H "Authorization: Bearer $api_key" -H 'Content-Type: application/json' -d "{\"model\":\"test-model\",\"input\":\"again\",\"previous_response_id\":\"$unstored_id\"}")"
test "$status" = "404"

echo "responses-state-smoke-ok"

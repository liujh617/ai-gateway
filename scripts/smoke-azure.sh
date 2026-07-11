#!/usr/bin/env bash
set -euo pipefail

fake_addr="${AZURE_FAKE_ADDR:-127.0.0.1:19090}"
gateway_addr="${GATEWAY_AZURE_SMOKE_ADDR:-127.0.0.1:18083}"
gateway_key="azure-smoke-gateway-key"
tmpdir="$(mktemp -d)"
config="$tmpdir/config.json"
fake_log="$tmpdir/azure-fake.log"
gateway_log="$tmpdir/gateway.log"

cleanup() {
  status=$?
  if [ -n "${gateway_pid:-}" ]; then
    kill "$gateway_pid" 2>/dev/null || true
    wait "$gateway_pid" 2>/dev/null || true
  fi
  if [ -n "${fake_pid:-}" ]; then
    kill "$fake_pid" 2>/dev/null || true
    wait "$fake_pid" 2>/dev/null || true
  fi
  if [ "$status" -ne 0 ]; then
    echo "--- azure fake upstream log ---" >&2
    test ! -f "$fake_log" || sed 's/local-azure-test-key/[REDACTED]/g' "$fake_log" >&2
    echo "--- gateway log ---" >&2
    test ! -f "$gateway_log" || sed 's/local-azure-test-key/[REDACTED]/g' "$gateway_log" >&2
  fi
  rm -rf "$tmpdir"
  exit "$status"
}
trap cleanup EXIT

AZURE_FAKE_ADDR="$fake_addr" go run ./internal/testupstream/azurefake >"$fake_log" 2>&1 &
fake_pid=$!

for _ in $(seq 1 30); do
  if curl -fsS "http://$fake_addr/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://$fake_addr/healthz" | grep -q '"status":"ok"'

cat >"$config" <<JSON
{
  "addr": "$gateway_addr",
  "api_key": "$gateway_key",
  "providers": {
    "azure": {
      "type": "azure-openai",
      "base_url": "http://$fake_addr",
      "api_key": "local-azure-test-key",
      "api_version": "2024-02-15-preview"
    }
  },
  "models": {
    "azure-chat": {
      "provider": "azure",
      "upstream_model": "chat-deployment",
      "capabilities": ["chat"]
    },
    "azure-embedding": {
      "provider": "azure",
      "upstream_model": "embedding-deployment",
      "capabilities": ["embeddings"]
    }
  }
}
JSON

GATEWAY_CONFIG="$config" GATEWAY_ADDR= GATEWAY_API_KEY= GATEWAY_API_KEYS= \
  go run ./cmd/gateway >"$gateway_log" 2>&1 &
gateway_pid=$!

for _ in $(seq 1 30); do
  if curl -fsS "http://$gateway_addr/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://$gateway_addr/healthz" | grep -q '"status":"ok"'

models="$(curl -fsS "http://$gateway_addr/v1/models" -H "Authorization: Bearer $gateway_key")"
grep -q '"id":"azure-chat"' <<<"$models"
grep -q '"id":"azure-embedding"' <<<"$models"

curl -fsS "http://$gateway_addr/v1/chat/completions" \
  -H "Authorization: Bearer $gateway_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"azure-chat","messages":[{"role":"user","content":"hello"}]}' \
  | grep -q '"object":"chat.completion"'

stream="$(curl -fsS -N "http://$gateway_addr/v1/chat/completions" \
  -H "Authorization: Bearer $gateway_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"azure-chat","stream":true,"messages":[{"role":"user","content":"hello"}]}')"
grep -q '"object":"chat.completion.chunk"' <<<"$stream"
grep -q 'data: \[DONE\]' <<<"$stream"

curl -fsS "http://$gateway_addr/v1/embeddings" \
  -H "Authorization: Bearer $gateway_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"azure-embedding","input":"hello"}' \
  | grep -q '"object":"list"'

echo "smoke-azure-ok"

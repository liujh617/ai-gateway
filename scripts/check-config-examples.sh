#!/usr/bin/env bash
set -euo pipefail

go test ./internal/config -run 'TestExampleConfigsLoad|TestLoadConfig'

GATEWAY_CONFIG=config.local.example.json GATEWAY_CHECK_CONFIG=1 go run ./cmd/gateway >/dev/null
GATEWAY_CONFIG=config.deepseek.example.json GATEWAY_CHECK_CONFIG=1 go run ./cmd/gateway >/dev/null

echo "config-examples-ok"

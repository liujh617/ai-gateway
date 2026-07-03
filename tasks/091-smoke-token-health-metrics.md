# Task 091 - Smoke token and health metrics

## 状态

Done.

## 背景

`scripts/smoke-fake.sh` 已覆盖 `/metrics` endpoint，但只检查 HTTP metrics 名称。随着 token usage metrics 和 provider health metrics 成为第一阶段核心观测能力，fake smoke test 可以在完成 chat completions 和 embeddings 请求后，轻量确认这些稳定指标已经出现。

## 目标

- 在 fake smoke test 末尾再次读取 `/metrics`。
- 验证 `open_ai_gateway_tokens_total` 出现。
- 验证 `open_ai_gateway_provider_health_status` 出现。
- 不强制触发 rate limit、fallback 或 circuit-open 行为，避免 smoke test 变慢或易抖动。

## 验收

- WSL `Ubuntu-24.04` `make smoke` 通过。
- WSL `Ubuntu-24.04` `make verify` 通过。

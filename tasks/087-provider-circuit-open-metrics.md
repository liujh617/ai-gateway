# Task 087 - Provider circuit open metrics

## 状态

Done.

## 背景

Provider health circuit breaker 已经会在 provider unhealthy 时跳过该 provider，并通过 health gauge 暴露当前状态。fallback metrics 也能看到请求最终切换到备用 provider，但缺少一个直接指标来统计“因为 circuit open 而跳过 provider”的次数。

## 目标

- 新增 `open_ai_gateway_provider_circuit_open_total` counter。
- 在 chat completions、stream chat completions 建连阶段和 embeddings 跳过 unhealthy provider 时递增。
- 标签包含规范化 `path`、external `model`、`provider` 和非敏感 gateway `client`。
- 不记录 API key、Authorization header、prompt 或 completion。

## 验收

- chat completions 跳过 unhealthy primary provider 时产生 circuit-open 指标。
- 既有 provider fallback 和 provider health 指标语义不变。
- WSL `Ubuntu-24.04` `make verify` 通过。

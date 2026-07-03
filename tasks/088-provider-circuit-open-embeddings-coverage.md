# Task 088 - Provider circuit open embeddings coverage

## 状态

Done.

## 背景

Task 087 新增了 provider circuit-open counter，并在 chat completions、stream chat completions 建连阶段和 embeddings 跳过 unhealthy provider 时递增。初始验收测试覆盖了 chat completions，但 embeddings 路径缺少直接断言。

## 目标

- 在 embeddings 跳过 unhealthy provider 的测试中断言 circuit-open metrics。
- 保持现有 fallback 和 health 指标断言。

## 验收

- embeddings 跳过 unhealthy primary provider 时产生 `open_ai_gateway_provider_circuit_open_total`。
- WSL `Ubuntu-24.04` `make verify` 通过。

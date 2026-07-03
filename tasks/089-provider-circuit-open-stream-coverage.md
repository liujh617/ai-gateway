# Task 089 - Provider circuit open stream coverage

## 状态

Done.

## 背景

Task 087 的 provider circuit-open 指标覆盖了 stream chat completions 建连阶段，但测试覆盖先补了非流式 chat 和 embeddings。流式建连阶段是 fallback 行为最容易退化的路径之一，需要单独锁定。

## 目标

- 增加 stream chat completions 跳过 unhealthy provider 的测试。
- 验证 primary provider 熔断后不会再次建连。
- 验证 stream 路径产生 `open_ai_gateway_provider_circuit_open_total`。

## 验收

- 连续 stream 建连失败触发熔断后，后续 stream 请求跳过 primary provider。
- 产生 path 为 `/v1/chat/completions` 的 circuit-open 指标。
- WSL `Ubuntu-24.04` `make verify` 通过。

# Task 065: Provider Fallback Metrics

## 状态

Done

## 背景

Task 063 已经支持按模型配置 provider fallback。为了判断 fallback 是否频繁发生、哪些 provider 组合不稳定，需要在 `/metrics` 中暴露 fallback 计数，同时保持标签集合可控。

## 范围

实现：

- 新增 `open_ai_gateway_provider_fallbacks_total`。
- 非流式 chat completions fallback 时递增计数。
- embeddings fallback 时递增计数。
- 流式 chat completions 建连阶段 fallback 时递增计数。
- 指标标签包含 `path`、对外 `model`、`from_provider` 和 `to_provider`。
- 不可 fallback 的客户端错误不递增计数。
- 更新兼容契约、架构概览和 README。

暂不实现：

- 按错误类型拆分 fallback 指标。
- provider 健康状态或熔断指标。
- fallback latency 分布。
- 已开始 SSE 响应后的 fallback，因为该场景不会切换 provider。

## 验收标准

- 普通 chat completions 触发 fallback 后，`/metrics` 暴露对应 fallback counter。
- embeddings 触发 fallback 后，`/metrics` 暴露对应 fallback counter。
- 流式 chat completions 建连失败触发 fallback 后，`/metrics` 暴露对应 fallback counter。
- 客户端请求错误类 provider error 不产生 fallback counter。
- `make verify`、`make build`、`make check-config-examples`、`make smoke` 和无 key 的 `make smoke-deepseek` 在 WSL `Ubuntu-24.04` 中通过。

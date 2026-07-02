# Task 067: Streaming Usage Metrics

## 状态

Done

## 背景

Task 064 已经为非流式 chat completions 和 embeddings 记录 provider 返回的 `usage` token 指标。部分 OpenAI-compatible upstream 会在流式 chat completions 的最终 SSE chunk 中返回 `usage`，网关应在不估算 token、不解析 prompt 或 completion 内容的前提下记录这类指标。

## 范围

实现：

- 在 `ChatCompletionChunk` 中保留可选 `usage` 字段。
- OpenAI-compatible provider 解析 SSE chunk 时保留 `usage`。
- HTTP API 转发流式 chunk 时，如果 chunk 包含 `usage`，记录 `open_ai_gateway_tokens_total`。
- 指标标签沿用 `path`、对外 `model`、实际命中的 `provider` 和 token `type`。
- 不包含 `usage` 的流式响应不记录 token 指标。
- 更新兼容契约、架构概览和 README。

暂不实现：

- token 估算。
- 从 delta 内容推断 token。
- 多个流式 usage chunk 的去重策略；当前按 provider 明确返回的 `usage` 累加。
- 成本金额计算。

## 验收标准

- 流式 chat completions 返回带 `usage` 的 SSE chunk 时，客户端响应保留该 `usage` 字段。
- `/metrics` 暴露对应 prompt、completion 和 total token counter。
- 指标使用对外模型名和实际 provider 名。
- 不带 `usage` 的流式响应不产生 token 指标。
- `make verify`、`make build`、`make check-config-examples`、`make smoke` 和无 key 的 `make smoke-deepseek` 在 WSL `Ubuntu-24.04` 中通过。

# Task 064: Token Usage Metrics

## 状态

Done

## 背景

网关已经提供基础 HTTP request metrics。随着真实 provider 接入和 fallback 能力落地，需要在不解析 prompt 或 completion 内容的前提下，暴露 provider 明确返回的 token usage，便于开发者观察模型调用量和后续成本统计。

## 范围

实现：

- 在 `/metrics` 中新增 `open_ai_gateway_tokens_total`。
- 仅使用 provider 响应中的 `usage` 字段，不做 token 估算。
- 覆盖非流式 chat completions 和 embeddings。
- 指标标签包含 `path`、对外 `model`、实际命中的 `provider` 和 token `type`。
- 不记录值为 `0` 的 token 样本。
- 更新兼容契约、架构概览和 README。

暂不实现：

- 流式响应 token usage 统计。
- 成本金额计算。
- 按租户、API key 或用户维度的统计。
- 持久化指标存储。

## 验收标准

- 非流式 chat completions 成功响应后，`/metrics` 暴露 prompt、completion 和 total token counter。
- embeddings 成功响应后，`/metrics` 暴露 prompt 和 total token counter。
- 指标使用对外模型名和实际命中的 provider。
- `make verify`、`make build`、`make check-config-examples`、`make smoke` 和无 key 的 `make smoke-deepseek` 在 WSL `Ubuntu-24.04` 中通过。

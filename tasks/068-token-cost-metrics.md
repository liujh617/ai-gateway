# Task 068: Token Cost Metrics

## 状态

Done

## 背景

Task 064 和 Task 067 已经在非流式与部分流式响应中记录 provider 明确返回的 token usage。为了便于本地观察模型调用成本，需要在不解析 prompt/completion 内容、不估算 token 的前提下，基于配置单价和 provider 返回的 `usage` 暴露基础成本指标。

## 范围

实现：

- 在模型配置和 fallback 配置中新增可选 `pricing`。
- `pricing.prompt_usd_per_1m_tokens` 表示 prompt token 的 USD / 1M tokens 单价。
- `pricing.completion_usd_per_1m_tokens` 表示 completion token 的 USD / 1M tokens 单价。
- 主 provider 和 fallback provider 可以配置不同 pricing。
- 在 `/metrics` 中新增 `open_ai_gateway_token_cost_usd_total`。
- 成本指标标签包含 `path`、对外 `model`、实际命中的 `provider` 和 token `type`。
- 仅当 provider 返回 `usage` 且配置了正数 pricing 时记录成本。
- 覆盖非流式 chat completions、embeddings，以及带 `usage` 的流式 chat completions。
- 更新兼容契约、架构概览、README 和 JSON Schema。

暂不实现：

- token 估算。
- 从 prompt 或 completion 内容推断成本。
- 持久化账单。
- 按租户、API key 或用户维度归集成本。
- 自动同步 provider 官方价格。

## 验收标准

- 配置 pricing 后，成功响应的 provider-reported usage 会产生 prompt、completion 和 total cost metrics。
- 未配置 pricing 或 pricing 为 `0` 时，不产生对应成本样本。
- fallback 命中时使用 fallback route 的 pricing 和 provider 标签。
- 负数 pricing 被配置校验拒绝。
- `make verify`、`make build`、`make check-config-examples`、`make smoke` 和无 key 的 `make smoke-deepseek` 在 WSL `Ubuntu-24.04` 中通过。

# Task 063: Provider Fallback

## 状态

Done

## 背景

第一阶段已经具备静态模型路由和 OpenAI-compatible provider。下一阶段需要在不引入复杂调度策略的前提下，支持简单、可预测的备用 provider：当主 provider 出现临时性上游错误时，网关可以尝试配置中的备用 provider，提高本地代理和真实 provider 接入时的可用性。

## 范围

实现：

- 在 `models.<external>.fallbacks` 中声明备用 provider 和可选 `upstream_model`。
- 非流式 chat completions 和 embeddings 在主 provider 返回可重试错误时尝试 fallback。
- 流式 chat completions 仅在建连阶段失败时尝试 fallback；一旦开始发送 SSE，不再切换 provider。
- `429`、`5xx`、timeout 和非兼容错误可触发 fallback。
- `400`、`401`、`404` 等客户端或鉴权类错误不触发 fallback。
- 更新配置 schema、兼容契约、架构概览和 README。

暂不实现：

- weighted routing。
- provider 健康状态缓存。
- 熔断、半开恢复或自动摘除。
- 单次请求内跨 provider 合并流式响应。

## 验收标准

- 配置校验接受合法 `fallbacks`，拒绝未知 fallback provider。
- 普通 chat completions 主 provider 失败时会调用 fallback。
- embeddings 主 provider 失败时会调用 fallback。
- 流式 chat completions 建连失败时会调用 fallback 并正常发送 `[DONE]`。
- 客户端请求错误类 provider error 不触发 fallback。
- `make verify`、`make build`、`make check-config-examples`、`make smoke` 和无 key 的 `make smoke-deepseek` 在 WSL `Ubuntu-24.04` 中通过。

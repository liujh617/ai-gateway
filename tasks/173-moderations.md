# Task 173 - Moderations

## 状态

Done.

## 背景

网关已支持 Chat Completions、Completions、Embeddings、Images、Responses 和 Models，缺少内容审核能力。`POST /v1/moderations` 是 OpenAI API 的审核端点。

## 变更

- 新增 `POST /v1/moderations` 端点。
- 新增 `compat.ModerationRequest` 和 `ModerationResponse` 类型。
- `Provider` 接口新增 `CreateModeration` 方法。
- fake、openai、azureopenai 实现 CreateModeration。
- 新增 `moderations` capability。
- 复用 model router、fallback、circuit breaker、metrics、audit。

## 验收

- JSON 响应返回 OpenAI-compatible 格式。
- 缺失 model、空 input 返回 `400 invalid_request_error`。
- 鉴权失败返回 `401`。
- WSL `make verify` 通过。

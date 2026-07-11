# Task 144 - Agent Audit Chat

## 背景

Audit recorder 已经接入 API server。本任务为非流式 `/v1/chat/completions` 写入完整请求与响应审计事件。

## 变更

- 新增 API audit 基础事件 helper。
- 非流式 chat 请求在路由解析后记录 `request` 事件。
- 非流式 chat 成功响应记录 `response` 事件。
- 响应事件包含最终命中的 provider、upstream model、HTTP status 和完整响应 body。
- fallback 调用返回实际命中的 provider 与 upstream model，供审计使用。

## 验证

- `go test ./internal/api -run TestAuditChatCompletionsNonStream -count=1`
- `go test ./internal/api -count=1`


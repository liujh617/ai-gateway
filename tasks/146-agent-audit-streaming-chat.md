# Task 146 - Agent Audit Streaming Chat

## 背景

非流式 chat 和 embeddings 已记录完整请求与响应。本任务为流式 `/v1/chat/completions` 增加 chunk、done 和流错误审计事件。

## 变更

- 流式 chat 复用已有 `request` 事件。
- 每个 SSE chunk 写入 `stream_chunk` 事件。
- 正常 EOF 写入 `stream_done` 事件。
- 流式取消、超时和非 EOF 错误写入 `error` 事件。
- stream fallback 返回实际 provider 和 upstream model，供审计事件标注。

## 验证

- `go test ./internal/api -run TestAuditChatCompletionsStream -count=1`
- `go test ./internal/api -count=1`


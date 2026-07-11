# Task 145 - Agent Audit Embeddings

## 背景

非流式 chat 已记录请求与响应。本任务为 `/v1/embeddings` 增加相同的 JSONL audit 事件。

## 变更

- embeddings 请求在路由解析后记录 `request` 事件。
- embeddings 成功响应记录 `response` 事件。
- 响应事件包含最终命中的 provider、upstream model、HTTP status 和完整响应 body。
- fallback 调用返回实际命中的 provider 与 upstream model，供审计使用。

## 验证

- `go test ./internal/api -run TestAuditEmbeddings -count=1`
- `go test ./internal/api -count=1`


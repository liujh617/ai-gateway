# Task 147 - Agent Audit Errors

## 背景

成功请求和流式 chunk 已记录。本任务补齐模型请求错误事件，并确认 audit recorder 写入失败不会影响主请求流程。

## 变更

- 新增 `writeAuditedError`，返回 OpenAI-compatible JSON 错误响应后写入 `error` 审计事件。
- chat 与 embeddings 在请求已 decode 后的 validation、模型不可用、provider 错误路径记录 `error` 事件。
- 流式 chat 已覆盖取消、超时和 stream error。
- JSONL recorder 增加关闭后 `Record` 不 panic 的测试。

## 验证

- `go test ./internal/audit ./internal/api -run 'TestAuditChatCompletionsValidationError|TestJSONLRecorderWriteFailureDoesNotPanic' -count=1`
- `go test ./internal/audit ./internal/api -count=1`


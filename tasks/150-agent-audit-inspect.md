# Task 150 - Agent Audit Inspect

## 背景

JSONL audit 文件会记录完整请求和响应。研究时通常需要快速扫事件序列，但直接 `cat` 文件容易再次暴露完整 prompt、completion、tool schema 或 embedding 数据。

## 变更

- 新增 `audit-inspect [path]` 子命令。
- 输出 JSONL 摘要，包含 event、request id、trace id、path、client、model、provider、status 和 `body_bytes`。
- 摘要不输出完整 `body` 内容。
- 本地验证文档增加命令示例。

## 验证

- `go test ./cmd/gateway -run TestRunAuditInspectSummarizesEventsWithoutBody -count=1`
- `go test ./cmd/gateway -count=1`


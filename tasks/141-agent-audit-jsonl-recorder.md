# 141 Agent Audit JSONL Recorder

## 背景

Agent audit JSONL mode 需要一个独立的审计写入组件，用于把完整请求/响应事件写入本地 JSONL 文件，同时保持 API 和 provider 边界清晰。

## 变更

- 新增 `internal/audit` 包。
- 支持 JSONL append 写入并自动创建父目录。
- 支持并发写入串行化。
- 支持 disabled/noop recorder。
- 支持从 `X-Agent-Trace-Id` 推导 `trace_id`，缺失时回退到 gateway request id。

## 验证

- `go test ./internal/audit -count=1`
- `make verify`

# Task 151 - Agent Audit Rotation

## 背景

Agent audit JSONL 会记录完整请求和响应。长期研究时单个文件可能快速增长，需要一个默认关闭、标准库实现的轻量轮转机制。

## 变更

- `audit.JSONLRecorderOptions` 新增 `MaxFileBytes`。
- `NewJSONLRecorderWithOptions` 支持配置单文件大小上限。
- 达到上限时将当前文件轮转为 `<path>.1`，新事件写入新的当前文件。
- `audit.max_file_bytes` 和 `GATEWAY_AUDIT_MAX_FILE_BYTES` 接入配置。
- `config check` 输出 `audit_max_file_bytes`。
- 同步 schema、示例配置和文档。

## 验证

- `go test ./internal/audit -run TestJSONLRecorderRotatesWhenMaxFileBytesExceeded -count=1`
- `go test ./internal/audit ./internal/config ./cmd/gateway -count=1`
- `make verify`
- `make check-config-examples`


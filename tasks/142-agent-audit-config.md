# Task 142 - Agent Audit Config

## 背景

Agent audit JSONL recorder 已具备基础写入能力。本任务将审计能力接入配置层，但仍保持默认关闭，避免默认记录完整 prompt 或 completion。

## 变更

- 新增 `audit.enabled` 与 `audit.path` 配置项。
- 默认值：
  - `audit.enabled=false`
  - `audit.path=audit/agent-trace.jsonl`
- 新增环境变量覆盖：
  - `GATEWAY_AUDIT_ENABLED`
  - `GATEWAY_AUDIT_PATH`
- `config check` 报告输出 `audit_enabled` 和 `audit_path`，不输出任何密钥。
- 同步更新 `schema/config.schema.json`。

## 验证

- `go test ./internal/config -count=1`


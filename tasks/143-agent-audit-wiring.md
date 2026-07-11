# Task 143 - Agent Audit Wiring

## 背景

Audit recorder 和配置层已经具备。本任务将 recorder 注入到 API server，并由 `cmd/gateway` 根据配置创建和关闭 recorder。

## 变更

- `api.Options` 新增 `Audit audit.Recorder`。
- `api.Server` 保存 audit recorder，未配置时使用 `audit.NoopRecorder`。
- `cmd/gateway` 根据 `audit.enabled` 创建 JSONL recorder。
- 服务退出时关闭 audit recorder。
- 启动日志输出 `audit_enabled` 与 `audit_path`。

## 验证

- `go test ./cmd/gateway ./internal/api`
- `make check-config`
- `make verify`


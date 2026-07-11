# Task 148 - Agent Audit Docs

## 背景

Agent audit JSONL 模式已经实现。本任务同步项目说明、外部契约、架构说明、示例配置和本地验证流程。

## 变更

- README 增加 audit 配置项和本地研究模式说明。
- OpenAI-compatible spec 增加 JSONL event 契约、trace header、env 覆盖和安全约束。
- 架构概览说明 audit 属于 API 层，不进入 provider adapter。
- 本地验证文档增加启用 audit 后运行 smoke 并查看 JSONL 的步骤。
- `config.example.json` 显式配置 `audit.enabled=false`。

## 验证

- `make check-config-examples`
- `go test ./internal/audit ./internal/config ./internal/api`
- `make verify`
- `make release-check`


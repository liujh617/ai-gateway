# Task 003: 配置校验与运行体验

## 状态

Done

## 背景

Task 002 已经支持真实 OpenAI-compatible upstream provider。Task 003 关注启动和运维体验：让错误配置尽早失败，让健康检查不需要鉴权，并让启动日志能说明当前加载了哪些 provider 和模型。

## 范围

实现：

- JSON 配置校验。
- `GET /healthz` 健康检查。
- `/healthz` 不需要 Bearer token。
- 启动日志输出 provider 和 model 摘要。
- `internal/config` 单元测试。
- 扩展配置示例。

暂不实现：

- 动态重载配置。
- provider 健康探测。
- 管理 API。
- 配置文件格式迁移到 YAML。

## 验收标准

- 缺少 provider 或 model 时启动失败。
- `openai-compatible` provider 缺少 `base_url` 时启动失败。
- `openai-compatible` provider 缺少 `api_key` 或 `api_key_env` 时启动失败。
- model 引用不存在的 provider 时启动失败。
- `GET /healthz` 在无 Authorization header 时返回 `200`。
- `go test ./...`、`go test -race ./...`、`go vet ./...` 在 WSL `Ubuntu-24.04` 中通过。


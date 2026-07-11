# Azure OpenAI Local Smoke Design

## 背景

网关已经支持 `azure-openai` provider，并有 provider 单元测试覆盖 deployment path、
`api-version`、`api-key`、非流式 chat、SSE chat 和 embeddings。但当前 CI 的服务级 smoke
只覆盖内置 fake provider、限流和 DeepSeek 无 key 跳过路径，没有从真实 gateway 进程经过
配置加载和路由再访问 Azure 风格 upstream 的端到端验证。

本任务增加一个完全本地、无需真实 Azure 凭据的 fake Azure upstream，并把对应 smoke 纳入
`release-check`。测试不得访问外网，也不得引入第三方依赖。

## 目标

- 通过真实 gateway 进程验证 `azure-openai` provider 的运行时 wiring。
- 验证非流式 chat、SSE streaming chat 和 embeddings 三条链路。
- 验证 Azure deployment endpoint、`api-version`、`api-key`、`Accept` 和请求体关键字段。
- 验证网关不会向 Azure upstream 发送 `Authorization` header。
- 将本地 Azure smoke 纳入 GitHub Actions 和 Gitee CI 使用的 `make release-check`。
- 所有实现只使用 Go 标准库、Bash 和现有 CI 已使用的 curl。

## 非目标

- 不访问真实 Azure OpenAI endpoint。
- 不要求或读取真实 Azure API key。
- 不修改 Azure provider、router、handler 或其他生产行为。
- 不模拟 Azure 的完整 API、鉴权系统、限流或错误目录。
- 不增加 Windows 原生 smoke；标准验证环境仍为 WSL `Ubuntu-24.04`。

## 方案概览

新增一个测试专用 Go 命令作为本地 Azure fake upstream，并新增一个 Bash smoke 脚本编排
fake upstream、gateway 和 curl 请求。fake upstream 对 Azure 契约做严格断言；gateway 对外响应
由 smoke 脚本断言。`Makefile` 将该 smoke 接入 `release-check`，现有 GitHub/Gitee workflow
无需新增 step。

## 组件设计

### Azure fake upstream

新增 `internal/testupstream/azurefake/main.go`。该命令仅用于仓库测试，不作为网关发布二进制。
它使用 `net/http` 监听 `AZURE_FAKE_ADDR`，默认 `127.0.0.1:19090`。

所有模型请求必须满足：

- `api-version` query 精确等于 `2024-02-15-preview`。
- `api-key` header 精确等于 `local-azure-test-key`。
- `Authorization` header 为空。
- `Content-Type` media type 为 `application/json`。

支持以下请求：

1. `POST /openai/deployments/chat-deployment/chat/completions`
   - body 的 `model` 必须为 `chat-deployment`。
   - `stream` 为 false 或缺省时，`Accept` 必须为 `application/json`，返回最小合法
     chat completion JSON。
   - `stream` 为 true 时，`Accept` 必须为 `text/event-stream`，返回一个合法 chunk 和
     `data: [DONE]`。
2. `POST /openai/deployments/embedding-deployment/embeddings`
   - body 的 `model` 必须为 `embedding-deployment`。
   - `Accept` 必须为 `application/json`。
   - 返回最小合法 embedding response JSON。
3. `GET /healthz`
   - 返回 `200` 和 `{"status":"ok"}`，供 smoke 脚本等待 fake upstream 就绪。

未知路径、错误 method、header、query 或 body 返回非 2xx；错误文本写入响应和 fake server log，
使 CI 失败时可以定位具体契约差异。日志不得输出 API key 明文；错误只描述 header 缺失或不匹配。

### Smoke 编排脚本

新增 `scripts/smoke-azure.sh`，使用 `set -euo pipefail`。

脚本行为：

1. 创建临时目录，并在其中写入 gateway config、两个进程日志及必要的临时响应文件。
2. 选择独立默认端口：fake upstream `127.0.0.1:19090`，gateway
   `127.0.0.1:18083`；允许分别通过 `AZURE_FAKE_ADDR` 和 `GATEWAY_AZURE_SMOKE_ADDR` 覆盖。
3. 启动 `go run ./internal/testupstream/azurefake`。
4. 轮询 fake upstream `/healthz`，最多 30 秒。
5. 生成 gateway config：
   - gateway key 为 `azure-smoke-gateway-key`。
   - provider type 为 `azure-openai`。
   - provider base URL 指向 fake upstream。
   - provider API key 为明显的本地占位值 `local-azure-test-key`。
   - API version 为 `2024-02-15-preview`。
   - 外部模型分别映射到 `chat-deployment` 和 `embedding-deployment`。
6. 启动真实 gateway 进程并轮询 `/healthz`，最多 30 秒。
7. 调用 `/v1/models`，确认两个外部模型可见。
8. 调用非流式 `/v1/chat/completions`，确认返回 `chat.completion`。
9. 调用流式 `/v1/chat/completions`，确认出现 chunk 和 `data: [DONE]`。
10. 调用 `/v1/embeddings`，确认返回 `object: list`。
11. 输出 `smoke-azure-ok`。

脚本通过 `trap` 始终终止 gateway 和 fake upstream，并删除临时目录。如果任一步失败，先输出两个
进程日志再以非零状态退出，避免 CI 只看到 curl 失败而没有上游断言原因。

### Makefile 与 CI

`Makefile` 增加 phony target：

```make
smoke-azure:
	bash scripts/smoke-azure.sh
```

`release-check` 在 `smoke-rate-limit` 后执行 `smoke-azure`，并继续保留
`smoke-deepseek-skip`。GitHub Actions 和 Gitee workflow 当前都运行 `make release-check`，因此
无需直接修改 workflow 文件。

## 文档与任务同步

- 更新 `docs/ci.md`：在 release-check 列表和说明中加入 `make smoke-azure`。
- 更新 `docs/local-verification.md`：补充本地 Azure fake upstream smoke 命令。
- 新增 `tasks/154-azure-local-smoke.md`：记录状态、范围与验收命令。

本变更不修改外部 API 契约，因此无需更新 `openai-compatible-proxy-spec.md` 或
`architecture/overview.md`。

## 测试与验证

在 WSL `Ubuntu-24.04` 中执行：

```bash
make smoke-azure
make release-check
```

验收条件：

- `make smoke-azure` 不要求环境中存在 Azure key，且不访问外网。
- fake upstream 确认三类请求使用正确 path、query、headers 和 deployment model。
- gateway 对外的非流式、SSE 和 embeddings 响应均通过 smoke 断言。
- 任一契约断言失败时脚本返回非零，并输出 fake upstream 与 gateway 日志。
- `make release-check` 全部通过。

## 风险与控制

- 固定端口可能与本地进程冲突。脚本提供环境变量覆盖，并为 Azure smoke 使用与其他 smoke
  不同的默认端口。
- 双进程编排容易遗留进程。脚本在启动第一个进程前安装 trap，并分别跟踪两个 PID。
- fake upstream 可能变成第二套 provider 实现。它只接受三个固定 endpoint 和最小字段，保持
  严格、短小，不抽象通用 Azure 模拟框架。
- CI 失败诊断可能被后台日志隐藏。失败 trap 明确输出两个日志，正常成功时不输出敏感配置。

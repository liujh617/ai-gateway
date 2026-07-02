# open-ai-gateway

`open-ai-gateway` 是一个基于 Go 的 OpenAI-compatible API 代理。它对外提供接近 OpenAI API 的 HTTP 契约，对内连接一个或多个上游模型服务，并在中间层处理模型映射、鉴权、流式响应、错误转换和可观测性。

## 目标

- 让现有 OpenAI SDK、CLI 和应用以最小改动接入网关。
- 支持 `/v1/chat/completions` 的非流式和 SSE 流式响应。
- 隔离上游 provider 的鉴权、模型命名、错误格式和响应差异。
- 使用 Go 构建简单、可部署、可观测的服务端代理。
- 为多 provider 路由、限流、配额、审计和成本统计预留扩展点。

## 文档路线

1. [README.md](README.md): 项目入口、目标和推进顺序。
2. [openai-compatible-proxy-spec.md](openai-compatible-proxy-spec.md): OpenAI-compatible 代理契约。
3. [architecture/overview.md](architecture/overview.md): 架构概览、模块边界和请求流程。
4. [AGENTS.md](AGENTS.md): 面向协作 agent 的工程约束。
5. [tasks/001-chat-completions.md](tasks/001-chat-completions.md): 第一阶段 chat completions 实现任务。

相关决策见 [ADR 0001](docs/adr/0001-go-openai-compatible-proxy.md)。

## 测试验证环境

项目标准测试与验证环境是 WSL 中的 `Ubuntu-24.04`。所有 Go 构建、测试、race 测试、vet 和手工 API 验证都应在该环境中执行。

详见 [Testing Environment](docs/testing-environment.md)。

## 开发命令

在 WSL `Ubuntu-24.04` 中运行：

```bash
make verify
make run
make check-config
make check-config-examples
```

本地服务级验证见 [Local Verification](docs/local-verification.md)。

部署和容器化见 [Deployment](docs/deployment.md)。

CI 自动化验证见 [CI](docs/ci.md)。

发布流程见 [Release](docs/release.md)。

## 本地运行

默认无配置启动时使用 fake provider，监听 `127.0.0.1:8080`，测试 API key 为 `test-gateway-key`，测试模型为 `test-model`。

在 WSL `Ubuntu-24.04` 中启动：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go run ./cmd/gateway"
```

非流式请求：

```bash
curl -sS http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer test-gateway-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"test-model","messages":[{"role":"user","content":"hello"}]}'
```

流式请求：

```bash
curl -N http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer test-gateway-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"test-model","stream":true,"messages":[{"role":"user","content":"hello"}]}'
```

## 接入真实上游

项目支持通过 JSON 配置接入 OpenAI-compatible upstream endpoint。示例见 [config.example.json](config.example.json)。

DeepSeek 和 Trae 本地代理示例见 [config.deepseek.example.json](config.deepseek.example.json) 与 [Trae + DeepSeek Local Proxy](docs/trae-deepseek.md)。

启动真实 provider：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "OPENAI_API_KEY=<your-key> GATEWAY_CONFIG=config.example.json go run ./cmd/gateway"
```

配置结构：

- `api_key`: 网关客户端 Bearer token，保留用于单 key 和早期配置。
- `api_keys`: 网关客户端 Bearer token 列表；非空时任一 token 都可通过鉴权。
- `api_clients`: 带非敏感 `name` 的网关客户端 Bearer token 列表；`name` 会进入日志和 metrics 的 `client` 标签，`api_key` 不会输出；可用 `models` 限制该 client 可见模型，可用 `rate_limit.requests_per_minute` 覆盖该 client 的限流。
- `providers.<name>.type`: 当前支持 `fake` 和 `openai-compatible`。
- `providers.<name>.base_url`: OpenAI-compatible base URL，例如 `https://api.openai.com/v1`。
- `providers.<name>.api_key_env`: 上游 API key 所在环境变量名。
- `providers.<name>.api_key`: 上游 API key 明文值，仅建议本地开发使用。
- `models.<external>.provider`: 对外模型路由到哪个 provider。
- `models.<external>.upstream_model`: 转发给上游的真实模型名。
- `models.<external>.capabilities`: 模型能力，支持 `chat`、`embeddings`；未配置时默认都支持。
- `models.<external>.fallbacks`: 备用 provider 列表；主 provider 在 `429`、`5xx`、timeout 或非兼容错误时按顺序尝试。
- `request_timeout_seconds`: 非流式请求的 provider 调用超时。
- `stream_timeout_seconds`: 流式请求的最大持续时间。
- `read_header_timeout_seconds`: HTTP server 读取请求头超时。
- `read_timeout_seconds`: HTTP server 读取完整请求超时，`0` 表示关闭。
- `write_timeout_seconds`: HTTP server 写响应超时，`0` 表示关闭；流式场景建议保持关闭。
- `idle_timeout_seconds`: keep-alive 空闲连接超时。
- `shutdown_timeout_seconds`: 收到 SIGINT/SIGTERM 后的 graceful shutdown 等待时间。
- `max_request_body_bytes`: 请求体大小上限，默认 `1048576`；`0` 表示关闭限制。
- `log.format`: 日志格式，支持 `text` 或 `json`。
- `log.level`: 日志级别，支持 `debug`、`info`、`warn`、`error`。
- `rate_limit.requests_per_minute`: 按 gateway client 的简单内存限流，`0` 表示关闭；`api_clients[].rate_limit.requests_per_minute` 可覆盖单个 client，显式 `0` 表示该 client 关闭限流。
- `provider_health.failure_threshold`: provider 连续可 fallback 错误达到该次数后短暂熔断，默认 `2`。
- `provider_health.cooldown_seconds`: provider 熔断后的冷却时间，默认 `30`。
- `models.<external>.pricing.prompt_usd_per_1m_tokens`: 可选 prompt token 单价，用于成本指标，单位 USD / 1M tokens。
- `models.<external>.pricing.completion_usd_per_1m_tokens`: 可选 completion token 单价，用于成本指标，单位 USD / 1M tokens。
- `models.<external>.fallbacks[].pricing`: fallback provider 可配置独立单价。

配置自检：

```bash
GATEWAY_CONFIG=config.example.json make check-config
```

自检输出会用 snake_case JSON 字段显示 gateway client、provider/model 摘要和 warning，不会输出 API key 明文。

配置 JSON Schema 见 [schema/config.schema.json](schema/config.schema.json)。

健康检查：

```bash
curl -sS http://127.0.0.1:8080/healthz
```

## 第一阶段范围

第一阶段只实现最小可用代理：

- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/embeddings`
- chat completions streaming
- 单模型主 provider 配置和可选 fallback provider
- Bearer token 鉴权
- 基础日志、request id、HTTP metrics、provider-reported token metrics（含带 `usage` 的流式响应）、可选 token cost metrics、provider fallback metrics、provider health metrics、provider circuit breaker、超时和错误响应

不在第一阶段实现：

- 多租户配额
- 持久化成本统计、账单和租户维度成本归集
- 管理后台
- 完整 OpenAI API 覆盖

## 预期目录

```text
.
├── AGENTS.md
├── README.md
├── architecture/
│   └── overview.md
├── cmd/
│   └── gateway/
├── docs/
│   └── adr/
├── internal/
│   ├── api/
│   ├── compat/
│   ├── config/
│   ├── middleware/
│   ├── provider/
│   └── router/
├── openai-compatible-proxy-spec.md
└── tasks/
    └── 001-chat-completions.md
```

## 兼容性原则

- 外部 HTTP 契约优先兼容 OpenAI API 的常用字段和错误格式。
- 内部 provider 接口不直接泄露上游供应商结构。
- 暂不支持的字段必须明确处理：透传、忽略或返回兼容错误；当前 chat completions 和 embeddings 会透传未知请求字段。
- 流式响应必须使用标准 SSE，并在完成时发送 `[DONE]`。

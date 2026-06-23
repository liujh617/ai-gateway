# ADR 0001: 使用 Go 构建 OpenAI-compatible API 代理

## 状态

Accepted

## 日期

2026-06-23

## 相关文档

- [README.md](../../README.md)
- [OpenAI-compatible Proxy Spec](../../openai-compatible-proxy-spec.md)
- [Architecture Overview](../../architecture/overview.md)
- [Testing Environment](../testing-environment.md)
- [Task 001: Chat Completions](../../tasks/001-chat-completions.md)

## 背景

本项目需要提供一个 OpenAI-compatible API 代理层，用于在客户端和一个或多个上游模型服务之间转发请求。代理需要尽量兼容 OpenAI API 的常见调用方式，让现有 SDK、CLI、应用和内部服务可以通过较小改动接入。

核心目标包括：

- 对外暴露 OpenAI-compatible HTTP API，例如 `/v1/chat/completions`、`/v1/completions`、`/v1/embeddings`、`/v1/models`。
- 支持非流式与流式响应，尤其是 Server-Sent Events 格式的 chat completion streaming。
- 屏蔽上游供应商差异，包括鉴权、模型命名、错误格式、超时和响应字段差异。
- 为后续接入多个 provider、路由策略、限流、审计日志和观测能力预留清晰边界。
- 优先保证服务端运行效率、部署简单性和可维护性。

## 决策

使用 Go 作为 OpenAI-compatible 代理服务的主要实现语言。

服务采用分层结构：

- HTTP API 层负责 OpenAI-compatible 路由、请求解析、响应编码和流式传输。
- 兼容层负责定义内部统一请求和响应模型，并处理 OpenAI 字段兼容。
- Provider 层负责适配不同上游模型服务。
- Router 层负责根据模型名、租户、配置或策略选择 provider。
- Middleware 层负责鉴权、日志、限流、超时、panic recovery、request id 和 metrics。
- Config 层负责加载 provider、模型映射、密钥、超时和功能开关。

优先使用 Go 标准库和成熟、小型依赖：

- HTTP server 使用 `net/http`。
- 路由可使用标准库 `http.ServeMux`，如路由复杂度增加再引入轻量 router。
- JSON 编解码使用 `encoding/json`，仅在性能瓶颈明确后考虑替换。
- 配置优先支持环境变量和 YAML/TOML/JSON 配置文件之一，避免早期引入复杂配置系统。

## 方案细节

### API 兼容边界

代理首先兼容高频 OpenAI API：

- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/completions`
- `POST /v1/embeddings`

请求和响应字段以 OpenAI API 为外部契约。对于暂不支持的字段，应保留解析能力并明确传递、忽略或返回兼容错误，避免静默产生不可预期行为。

错误响应统一转换为 OpenAI-compatible 格式：

```json
{
  "error": {
    "message": "error message",
    "type": "invalid_request_error",
    "param": null,
    "code": null
  }
}
```

### 流式响应

流式 chat completion 使用 SSE：

- 设置 `Content-Type: text/event-stream`。
- 每个事件使用 `data: ...\n\n`。
- 结束时发送 `data: [DONE]\n\n`。
- 客户端断开时通过 request context 取消上游请求。

Provider 适配器必须显式声明是否支持 streaming。不支持 streaming 的 provider 不做伪流式包装，避免客户端误判真实生成延迟。

### Provider 抽象

Provider 接口以内部统一模型为边界，而不是直接暴露 OpenAI 请求结构：

```go
type Provider interface {
    ListModels(ctx context.Context) ([]Model, error)
    CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)
    StreamChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionStream, error)
    CreateEmbedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
}
```

这样可以避免 OpenAI 外部契约和上游供应商实现细节互相污染。

### 模型映射

对外模型名与上游模型名通过配置映射：

```yaml
models:
  gpt-4o-mini:
    provider: openai
    upstream_model: gpt-4o-mini
  qwen-plus:
    provider: dashscope
    upstream_model: qwen-plus
```

Router 根据外部模型名选择 provider，并将内部请求中的模型名替换为上游模型名。

### 鉴权

代理对外使用 Bearer token：

```http
Authorization: Bearer <gateway-api-key>
```

上游 provider 的 API key 不暴露给客户端，由服务端配置管理。后续可以扩展为多租户 key、配额、模型白名单和审计。

### 可观测性

每个请求应记录：

- request id
- route
- external model
- provider
- upstream model
- status code
- latency
- streaming flag
- error type

日志不得记录用户完整 prompt、完整响应内容或上游密钥。需要内容审计时应通过显式配置开启，并配套脱敏策略。

## 备选方案

### 使用 Node.js/TypeScript

优点是生态中 OpenAI SDK 和 Web API 体验较好，开发速度快。

缺点是长期运行代理服务时，需要更谨慎处理 streaming backpressure、部署体积、运行时版本和资源占用。对于以网关为核心的后端服务，Go 的单二进制部署和并发模型更直接。

### 使用 Python

优点是 AI 生态成熟，provider SDK 丰富。

缺点是作为高并发 API 代理时，性能、部署和异步流式处理复杂度更高。Python 更适合作为模型编排或离线任务层，而不是本项目的核心网关进程。

### 基于现有 API gateway 扩展

优点是可以复用成熟的限流、鉴权和观测能力。

缺点是 OpenAI-compatible 语义、SSE streaming、provider 差异适配和模型映射通常需要较多自定义插件。早期直接用 Go 实现业务网关更容易保持领域逻辑清晰。

## 后果

正面影响：

- Go 的 goroutine、context 和 `net/http` 适合实现高并发 HTTP 代理和流式转发。
- 单二进制部署降低运维复杂度。
- Provider 抽象有利于后续增加 OpenAI、Azure OpenAI、Anthropic-compatible gateway、DashScope、Gemini adapter 等上游。
- 内部统一模型可以隔离外部 OpenAI-compatible 契约和上游差异。

负面影响：

- 需要自行维护 OpenAI-compatible schema 和错误格式，必须通过兼容性测试降低漂移风险。
- Go 官方或社区 SDK 对不同模型供应商的封装不如 Python 生态集中。
- streaming、取消、超时和错误转换需要细致测试，否则容易出现连接泄漏或客户端兼容问题。

## 测试策略

- 单元测试覆盖请求解析、模型映射、错误转换和 provider 路由。
- 使用 fake provider 测试非流式和流式响应。
- 使用 OpenAI 官方 SDK 或兼容客户端做集成测试，验证最常见调用路径。
- 覆盖客户端取消、上游超时、上游 4xx/5xx、provider 不支持 streaming 等场景。
- 对 SSE 响应测试事件格式、`[DONE]` 结束标记和 flush 行为。

## 迁移与演进

第一阶段实现最小可用代理：

- `/v1/models`
- `/v1/chat/completions`
- streaming chat completions
- 单 provider 配置
- Bearer token 鉴权
- 基础日志和超时

第二阶段扩展：

- 多 provider 和模型映射
- embeddings
- 限流和配额
- metrics
- retry 和 fallback 策略

第三阶段增强：

- 多租户
- provider 健康检查
- 管理 API
- 请求审计和成本统计
- 更完整的 OpenAI-compatible 兼容测试套件

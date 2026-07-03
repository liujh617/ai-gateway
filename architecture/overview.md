# Architecture Overview

本文描述 `open-ai-gateway` 的第一阶段架构。详细决策见 [ADR 0001](../docs/adr/0001-go-openai-compatible-proxy.md)，外部 API 契约见 [OpenAI-compatible Proxy Spec](../openai-compatible-proxy-spec.md)。

## 架构目标

- 对外提供 OpenAI-compatible HTTP API。
- 对内通过 provider adapter 连接一个或多个上游模型服务。
- 用内部统一模型隔离 OpenAI 外部契约和上游供应商差异。
- 在网关层集中处理鉴权、模型映射、错误转换、日志、超时和取消。

## 模块边界

```text
Client
  |
  v
HTTP API
  |
  v
Middleware
  |
  v
Compat Mapper
  |
  v
Model Router
  |
  v
Provider Adapter
  |
  v
Upstream Model Service
```

### HTTP API

职责：

- 注册 `/v1/*` 路由。
- 解析请求 body。
- 根据 `stream` 字段选择 JSON 或 SSE 响应。
- 写入 OpenAI-compatible response。

不负责：

- provider 选择。
- 上游模型字段转换。
- 业务限流策略。

### Middleware

职责：

- request id。
- 复用合法的 `X-Request-Id`，并在非法或缺失时生成新的 request id。
- panic recovery。
- Bearer token 鉴权。
- 基础访问日志。
- 可配置日志格式和级别。
- HTTP metrics。
- 按 gateway client 做简单 in-memory 限流，并支持 per-client 覆盖。
- 设置基础安全响应头。
- 超时和 context 管理。
- SIGINT/SIGTERM graceful shutdown。
- metrics hook。

结构化 access log 字段：

- `request_id`
- `method`
- `path`
- `status`
- `latency_ms`
- `external_model`
- `client`
- `provider`
- `upstream_model`
- `stream`
- `error_type`
- `error_code`

Access log `client` uses the non-secret gateway client name. Public routes use `public`, authentication failures use `unauthenticated`, and disabled gateway auth uses `unconfigured`.

Access log `path` fields keep known routes and collapse unknown routes to `/__unknown__`.

Metrics endpoint:

- `GET /metrics`
- Prometheus text exposition format
- `open_ai_gateway_http_requests_total`
- `open_ai_gateway_http_request_duration_seconds_total`
- `open_ai_gateway_tokens_total`
- `open_ai_gateway_token_cost_usd_total`
- `open_ai_gateway_rate_limit_rejections_total`
- `open_ai_gateway_provider_fallbacks_total`
- `open_ai_gateway_provider_health_status`
- Metrics `path` labels keep known routes and collapse unknown routes to `/__unknown__`.
- HTTP metrics include the non-secret gateway client label when available.
- Token metrics use provider-reported `usage` only and are labeled by path, external model, provider, token type, and gateway client. Streaming chat completions record token metrics only when an upstream SSE chunk includes `usage`.
- Cost metrics use provider-reported `usage` plus optional per-model or fallback pricing config, and are labeled by path, external model, provider, token type, and gateway client.
- Rate limit rejection metrics count gateway-side rate limiter rejections and are labeled by normalized path and gateway client.
- Provider fallback metrics are labeled by path, external model, source provider, target provider, and gateway client.
- Provider health metrics expose each provider's in-memory circuit breaker state.

Runtime probes:

- `GET /healthz`: process liveness
- `GET /readyz`: readiness based on loaded model routes
- `GET /version`: build metadata
- `HEAD` for GET routes returns the same status and headers without a response body.

### Compat Mapper

职责：

- 将外部 OpenAI-compatible request 转换为内部 request。
- 将内部 response 转换回 OpenAI-compatible response。
- 将内部错误转换为 OpenAI-compatible error response。

Compat Mapper 是外部 API 契约的主要守门员。

### Model Router

职责：

- 根据对外模型名查找 provider。
- 将对外模型名映射为上游模型名。
- 校验模型是否支持当前 API 能力，例如 `chat` 或 `embeddings`。
- 按模型配置提供主 provider 和 fallback provider 尝试顺序。
- 携带每个 provider 尝试对应的可选 token pricing。
- 返回明确的 model not found 或 unauthorized 错误。

当前实现使用静态配置路由，并支持在非流式请求或流式建连阶段按顺序尝试 fallback provider。provider 连续出现可 fallback 错误后会短暂熔断，后续请求会跳过 unhealthy provider，冷却结束后再次尝试。后续可以扩展 weighted routing 和灰度策略。

### Provider Adapter

职责：

- 处理上游 API key。
- 发起上游 HTTP 请求。
- 转换上游请求和响应。
- 处理上游错误、超时和取消。
- 将 models、非流式 chat completions、embeddings，以及流式 chat completions 建连和读取阶段的上游 transport timeout 规范化为 context deadline exceeded，便于 HTTP API 映射为 `504`。
- 转发网关最终使用的 request id，便于上游日志关联。
- 设置稳定的上游 `User-Agent`，便于上游识别网关调用。
- 设置明确的上游 `Accept` header，区分 JSON 和 SSE 响应协商。
- 仅在带 JSON body 的上游请求上设置 `Content-Type: application/json`。
- 校验非流式 JSON 上游成功响应的 `Content-Type`。
- 校验流式上游成功响应的 `Content-Type`。
- 仅解析 `Content-Type: application/json` 的上游错误响应字段。
- 限制单个上游 SSE event 大小，避免异常流式事件占用过多内存。
- 以分片方式读取上游 SSE line，并限制单行大小。
- 支持 LF、CRLF 和 CR 三种上游 SSE line ending，CR 结束的行不等待后续字节。
- 拒绝未以空行结束的上游 SSE event。
- 按 SSE 规则处理没有冒号的 field line。
- 忽略上游 SSE stream 第一行开头的 UTF-8 BOM。
- 暴露是否支持 streaming 的能力信息。

Provider Adapter 不应直接依赖 HTTP handler。

当前实现：

- `internal/provider/fake`: 开发和测试用 fake provider。
- `internal/provider/openai`: OpenAI-compatible HTTP provider，转发 `/chat/completions`，支持非流式和 SSE 流式响应，并对上游成功响应和 JSON 错误响应执行大小边界及单 JSON 校验。

### Config

职责：

- 加载监听地址。
- 加载一个或多个网关 API key，支持非敏感 gateway client name。
- 加载可选的 gateway client 模型白名单。
- 加载全局限流和可选的 gateway client 限流覆盖。
- 加载 provider 配置。
- 加载模型映射。
- 加载模型和 fallback 的可选 token pricing。
- 加载超时配置。
- 加载 provider health / circuit breaker 配置。
- 通过共享的 upstream URL 校验逻辑，校验 OpenAI-compatible provider `base_url` 只使用 `http` 或 `https`，且不包含 query 或 fragment。

当前实现使用 JSON 配置文件，并通过 `GATEWAY_CONFIG` 指定路径。无配置时使用 fake provider 默认配置。
配置自检以 snake_case JSON 字段输出 gateway client、provider 和 model 摘要；gateway client 摘要只包含非敏感名称、模型白名单和限流覆盖，不输出 API key。

## 请求流程

### Non-stream Chat Completion

1. Client 调用 `POST /v1/chat/completions`。
2. Middleware 生成 request id，验证 Bearer token。
3. HTTP API 解析 JSON。
4. Compat Mapper 转换为内部 chat request。
5. Model Router 根据 `model` 选择 provider 和上游模型。
6. Provider Adapter 调用上游服务。
7. Compat Mapper 转换为 OpenAI-compatible response。
8. HTTP API 返回 JSON。

### Stream Chat Completion

1. Client 调用 `POST /v1/chat/completions`，并设置 `"stream": true`。
2. Middleware 建立可取消 context。
3. HTTP API 解析请求，准备 SSE headers。
4. Model Router 选择 provider。
5. Provider Adapter 打开上游 stream。
6. HTTP API 逐个写入 `data: ...\n\n` 并 flush。
7. 上游结束后写入 `data: [DONE]\n\n`。
8. Client 断开或请求超时时，context 取消并关闭上游 stream。

## 代码目录建议

```text
cmd/gateway/
  main.go

internal/api/
  server.go
  chat_completions.go
  provider_health.go

internal/compat/
  types.go
  errors.go

internal/config/
  config.go

internal/provider/
  provider.go
  fake/
  openai/

internal/router/
  model_router.go

internal/routes/
  routes.go

internal/middleware/
  auth.go
  logging.go
  request_id.go
  recovery.go
```

## 关键工程约束

- 已知 HTTP 路由、公开访问属性和允许 method 的元数据必须集中维护，HTTP mux 必须从该 metadata 迭代注册，并供鉴权、限流绕过、405 判断、`Allow` header、metrics path 归一化和 access log path 归一化复用。
- 所有上游请求必须使用 incoming request context。
- 普通请求必须使用 request timeout。
- 流式请求必须使用独立 stream timeout，不能被普通 request timeout 截断。
- HTTP server 必须配置 ReadHeaderTimeout。
- 关闭进程时必须优先尝试 graceful shutdown。
- 所有 response body 和 stream 必须关闭。
- 上游 JSON 响应必须按单一 JSON 值严格解析。
- SSE handler 必须检查 `http.Flusher`。
- 日志不得记录完整 prompt、完整 completion 或任何 API key。
- 日志不得记录 `Authorization` header。
- OpenAI-compatible 类型变化必须同步更新 spec 和测试。

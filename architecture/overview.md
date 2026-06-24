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
- panic recovery。
- Bearer token 鉴权。
- 基础访问日志。
- 按 Bearer token 做简单 in-memory 限流。
- 超时和 context 管理。
- metrics hook。

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
- 返回明确的 model not found 或 unauthorized 错误。

第一阶段只需要静态配置路由。后续可以扩展 weighted routing、fallback 和灰度策略。

### Provider Adapter

职责：

- 处理上游 API key。
- 发起上游 HTTP 请求。
- 转换上游请求和响应。
- 处理上游错误、超时和取消。
- 暴露是否支持 streaming 的能力信息。

Provider Adapter 不应直接依赖 HTTP handler。

当前实现：

- `internal/provider/fake`: 开发和测试用 fake provider。
- `internal/provider/openai`: OpenAI-compatible HTTP provider，转发 `/chat/completions`，支持非流式和 SSE 流式响应。

### Config

职责：

- 加载监听地址。
- 加载网关 API key。
- 加载 provider 配置。
- 加载模型映射。
- 加载超时配置。

当前实现使用 JSON 配置文件，并通过 `GATEWAY_CONFIG` 指定路径。无配置时使用 fake provider 默认配置。

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

internal/middleware/
  auth.go
  logging.go
  request_id.go
  recovery.go
```

## 关键工程约束

- 所有上游请求必须使用 incoming request context。
- 普通请求必须使用 request timeout。
- 流式请求必须使用独立 stream timeout，不能被普通 request timeout 截断。
- 所有 response body 和 stream 必须关闭。
- SSE handler 必须检查 `http.Flusher`。
- 日志不得记录完整 prompt、完整 completion 或任何 API key。
- OpenAI-compatible 类型变化必须同步更新 spec 和测试。

# OpenAI-compatible Proxy Spec

本文定义 `open-ai-gateway` 第一阶段需要支持的 OpenAI-compatible API 契约。该契约面向客户端，内部实现可以使用不同的数据结构和 provider 适配器。

## 基本约定

### Base URL

客户端通过网关地址访问 API：

```text
https://gateway.example.com/v1
```

### 鉴权

所有受保护接口使用 Bearer token：

```http
Authorization: Bearer <gateway-api-key>
```

缺少、无效或无权限访问模型时，返回 OpenAI-compatible error response。

### Content Type

普通 JSON 请求：

```http
Content-Type: application/json
```

`POST` JSON 接口会校验 `Content-Type`，缺失或不是 `application/json` 时返回 `415 invalid_request_error`。允许标准参数，例如 `application/json; charset=utf-8`。

请求体必须只包含一个 JSON 值。合法 JSON 后继续拼接第二个 JSON 值或其他非空 token 时，网关返回 `400 invalid_request_error`。

流式响应：

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

### Request ID

客户端可以通过 `X-Request-Id` 传入请求标识。网关会在响应中返回最终使用的 `X-Request-Id`，在 access log 的 `request_id` 字段中记录，并转发给 OpenAI-compatible 上游 provider。

传入值会先去掉首尾空白；为空、长度超过 128 字节、包含空白/控制字符或非 ASCII 可见字符时，网关会生成新的 request id。

### Upstream Request Headers

OpenAI-compatible provider 调用上游时会发送稳定的 `User-Agent`，格式为 `open-ai-gateway/<version>`。版本值来自构建注入的 `version`，为空或非法字符会被规范化，默认使用 `dev`。

非流式 JSON 上游请求会发送 `Accept: application/json`。流式 chat completions 上游请求会发送 `Accept: text/event-stream`。

带 JSON body 的上游请求会发送 `Content-Type: application/json`。无请求体的上游 `GET /models` 不发送 `Content-Type`。

非流式 JSON 上游成功响应必须返回 `Content-Type: application/json`。允许标准参数，例如 `application/json; charset=utf-8`。不满足时，网关将其视为 provider 错误。

流式 chat completions 上游成功响应必须返回 `Content-Type: text/event-stream`。允许标准参数，例如 `text/event-stream; charset=utf-8`。不满足时，网关将其视为 provider 错误。

### Security Headers

所有响应都会包含：

```http
X-Content-Type-Options: nosniff
```

## 错误格式

错误响应统一为：

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

推荐映射：

| 场景 | HTTP status | type |
| --- | --- | --- |
| 请求 JSON 无效 | `400` | `invalid_request_error` |
| 缺少必填字段 | `400` | `invalid_request_error` |
| Content-Type 不支持 | `415` | `invalid_request_error` |
| 鉴权失败 | `401` | `authentication_error` |
| 方法不支持 | `405` | `invalid_request_error` |
| 未知路由 | `404` | `invalid_request_error` |
| 模型不存在或无权限 | `404` | `invalid_request_error` |
| provider 超时 | `504` | `server_error` |
| provider 返回限流 | `429` | `rate_limit_error` |
| provider 内部错误 | `502` | `server_error` |

OpenAI-compatible provider 的上游 transport timeout 会统一视为 `context deadline exceeded`，覆盖 models、非流式 chat completions、embeddings，以及流式 chat completions 建连和读取阶段，由 HTTP API 映射为 `504 server_error`。

OpenAI-compatible provider 会尽量保留 upstream error 中的 `message`、`type`、`param` 和 `code`。仅当上游错误响应 `Content-Type` 是 `application/json`，且响应体未超过网关读取上限并且是单个 JSON 值时，错误字段才会被采用；否则错误体会被视为不可信并回退到默认错误映射。上游 `5xx` 会映射为网关 `502`，避免把上游内部状态直接暴露为网关自身故障。

OpenAI-compatible provider 解析上游 JSON 响应时也要求响应体只包含一个 JSON 值。合法 JSON 后继续拼接第二个 JSON 值或其他非空 token 时，网关将其视为 provider 错误。

未知受保护路由在鉴权通过后返回 JSON 格式的 `404 invalid_request_error`；未鉴权时仍先返回 `401 authentication_error`。

已知路由使用不支持的 HTTP method 时，在鉴权通过后返回 JSON 格式的 `405 invalid_request_error`，并设置 `Allow` header；未鉴权时仍先返回 `401 authentication_error`。

## `GET /v1/models`

返回网关对当前调用方可见的模型列表。

`HEAD /v1/models` 需要 Bearer token，返回相同状态码和响应头，但不返回 body。

### Response

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o-mini",
      "object": "model",
      "created": 0,
      "owned_by": "open-ai-gateway"
    }
  ]
}
```

## `GET /healthz`

返回网关进程健康状态。该接口不需要 Bearer token。

`HEAD /healthz` 返回相同状态码和响应头，但不返回 body，供负载均衡器和探针使用。

### Response

```json
{
  "status": "ok"
}
```

## `GET /readyz`

返回网关是否已准备好接收业务请求。该接口不需要 Bearer token。

`HEAD /readyz` 返回相同状态码和响应头，但不返回 body，供负载均衡器和探针使用。

当前 readiness 只检查网关是否加载了至少一个对外模型，不主动探测上游 provider，避免 readiness probe 引入外部依赖抖动。

### Ready Response

```json
{
  "status": "ready",
  "models": 1
}
```

### Not Ready Response

```json
{
  "status": "not_ready",
  "models": 0
}
```

## `GET /metrics`

返回 Prometheus text exposition 格式的基础 HTTP 指标。该接口不需要 Bearer token。

`HEAD /metrics` 返回相同状态码和响应头，但不返回 body。

当前指标：

- `open_ai_gateway_http_requests_total`
- `open_ai_gateway_http_request_duration_seconds_total`
- `open_ai_gateway_tokens_total`
- `open_ai_gateway_provider_fallbacks_total`

HTTP 指标标签：

- `method`
- `path`
- `status`

`path` 标签只保留已知路由。未知路由统一归一化为 `/__unknown__`，避免恶意或异常路径造成 Prometheus label cardinality 无界增长。

Token 指标仅统计上游响应中明确返回的 `usage`，不做估算。当前覆盖非流式 chat completions 和 embeddings；流式响应如未返回最终 usage，不记录 token 指标。

Token 指标标签：

- `path`
- `model`
- `provider`
- `type`: `prompt`、`completion` 或 `total`

Provider fallback 指标在请求从主 provider 切换到备用 provider 时递增。流式响应只统计建连阶段的 fallback；SSE 已开始发送后不会切换 provider，也不会产生 fallback 指标。

Provider fallback 指标标签：

- `path`
- `model`
- `from_provider`
- `to_provider`

## `GET /version`

返回当前二进制的构建信息。该接口不需要 Bearer token。

`HEAD /version` 返回相同状态码和响应头，但不返回 body。

### Response

```json
{
  "version": "dev",
  "commit": "unknown",
  "build_time": "unknown"
}
```

## `POST /v1/chat/completions`

创建 chat completion。第一阶段必须支持非流式和流式两种模式。

### Request

必须支持字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 是 | 对外模型名，由网关映射到上游模型 |
| `messages` | array | 是 | chat messages |
| `stream` | boolean | 否 | 是否使用 SSE 流式响应 |
| `temperature` | number | 否 | 采样温度 |
| `top_p` | number | 否 | nucleus sampling |
| `max_tokens` | integer | 否 | 最大输出 token 数 |
| `stop` | string/array | 否 | 停止序列 |
| `user` | string | 否 | 终端用户标识 |

`messages` 必须支持：

```json
[
  {
    "role": "system",
    "content": "You are helpful."
  },
  {
    "role": "user",
    "content": "Hello"
  }
]
```

第一阶段支持的 `role`：

- `system`
- `user`
- `assistant`
- `tool`

未在上表列出的请求字段会在兼容层按原始 JSON 保留，并在调用 provider 时透传给上游。常见字段包括 `tools`、`tool_choice`、`response_format`、`parallel_tool_calls`、`seed` 等。网关仍然会覆盖 `model` 为路由后的上游模型名；非流式 provider 调用会清除 `stream=true`，避免误触发上游流式响应。

### Non-stream Response

```json
{
  "id": "chatcmpl_gateway_123",
  "object": "chat.completion",
  "created": 1710000000,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 3,
    "total_tokens": 13
  }
}
```

### Stream Response

流式响应使用 SSE，每个事件格式为：

```text
data: {"id":"chatcmpl_gateway_123","object":"chat.completion.chunk","created":1710000000,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

```

结束事件：

```text
data: [DONE]

```

约束：

- 每个事件必须以空行结束；上游在事件中途 EOF 时，网关会将其视为 provider 错误。
- SSE line ending 支持 LF、CRLF 和 CR；CR 结束的行会在 CR 到达后立即完成，不等待后续字节。
- provider 适配器按 SSE event 解析，而不是按单行解析。
- 多行 `data:` 会按 SSE 规则合并。
- 没有冒号的 SSE field line 会按空值处理。
- 上游 SSE stream 第一行开头的 UTF-8 BOM 会被忽略。
- 单个上游 SSE event 超过网关响应体读取上限时，网关会将其视为 provider 错误。
- 单个上游 SSE line 超过网关响应体读取上限时，网关会在行读取阶段拒绝该流。
- comment、`event`、`id`、`retry` 行会被忽略。
- handler 必须在写入 chunk 后 flush。
- request context 取消时必须关闭上游流。
- 上游流异常时，如果响应头尚未发送，返回 JSON 错误；如果已经开始 SSE，则发送兼容的错误 chunk 或直接结束连接，并记录日志。

## `POST /v1/embeddings`

创建 embeddings。第一阶段支持 OpenAI-compatible embeddings 基础请求和响应。

### Request

必须支持字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 是 | 对外模型名，由网关映射到上游模型 |
| `input` | string/array | 是 | 输入文本或 token 数组 |
| `encoding_format` | string | 否 | 例如 `float` 或 `base64` |
| `user` | string | 否 | 终端用户标识 |

未在上表列出的请求字段会在兼容层按原始 JSON 保留，并在调用 provider 时透传给上游。常见字段包括 `dimensions` 等。网关仍然会覆盖 `model` 为路由后的上游模型名。

### Response

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.1, 0.2, 0.3]
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 1,
    "total_tokens": 1
  }
}
```

## 模型映射

客户端只感知对外模型名。当前实现使用 JSON 配置：

```json
{
  "providers": {
    "openai": {
      "type": "openai-compatible",
      "base_url": "https://api.openai.com/v1",
      "api_key_env": "OPENAI_API_KEY",
      "timeout_seconds": 60
    }
  },
  "models": {
    "gpt-4o-mini": {
      "provider": "openai",
      "upstream_model": "gpt-4o-mini",
      "capabilities": ["chat"],
      "fallbacks": [
        {
          "provider": "backup-openai",
          "upstream_model": "gpt-4o-mini"
        }
      ]
    },
    "qwen-plus": {
      "provider": "dashscope",
      "upstream_model": "qwen-plus",
      "capabilities": ["chat"]
    }
  }
}
```

早期 ADR 中的 YAML 示例属于目标形态。当前实现使用 JSON 配置，避免在第一版引入额外配置解析依赖。

目标 YAML 形态：

```yaml
models:
  gpt-4o-mini:
    provider: openai
    upstream_model: gpt-4o-mini
  qwen-plus:
    provider: dashscope
    upstream_model: qwen-plus
```

网关日志应同时记录 external model 和 upstream model，但不得记录上游 API key。

模型能力通过 `capabilities` 声明：

- `chat`: 可用于 `/v1/chat/completions`
- `embeddings`: 可用于 `/v1/embeddings`

未声明 `capabilities` 时默认同时支持 `chat` 和 `embeddings`，用于兼容早期配置。请求使用不支持该能力的模型时，返回 `404 invalid_request_error`。

模型可以通过 `fallbacks` 声明备用 provider。主 provider 在非流式请求或流式建连阶段返回 `429`、`5xx`、timeout 或非兼容错误时，网关会按配置顺序尝试备用 provider。`400`、`401`、`404` 等客户端或鉴权类错误不会触发 fallback。流式响应一旦开始向客户端发送事件，后续读取错误不会再切换 provider，避免混合两个上游的 SSE 响应。

OpenAI-compatible provider 的 `base_url` 会被统一规范化，去除首尾空白和末尾 `/`。规范化后的 `base_url` 必须是 `http` 或 `https` URL，且不能包含 query 或 fragment。配置校验和 provider 构造使用同一套规则。

## 超时

当前配置项：

```json
{
  "request_timeout_seconds": 60,
  "stream_timeout_seconds": 600,
  "read_header_timeout_seconds": 10,
  "read_timeout_seconds": 0,
  "write_timeout_seconds": 0,
  "idle_timeout_seconds": 120,
  "shutdown_timeout_seconds": 10,
  "max_request_body_bytes": 1048576
}
```

默认值：

- 普通请求总超时：60 秒
- 流式请求最大持续时间：10 分钟
- 上游连接超时：10 秒
- 请求头读取超时：10 秒
- keep-alive 空闲超时：120 秒
- graceful shutdown 超时：10 秒
- 请求体大小上限：1 MiB

所有超时应可配置。

请求体超过上限返回 `413 invalid_request_error`。

普通请求超时返回：

```json
{
  "error": {
    "message": "provider timeout",
    "type": "server_error",
    "param": null,
    "code": null
  }
}
```

## 限流

当前支持简单 in-memory rate limiter：

```json
{
  "rate_limit": {
    "requests_per_minute": 120
  }
}
```

约束：

- `0` 表示关闭限流。
- 限流 key 优先使用 Bearer token。
- `/healthz` 不参与限流。
- 超限返回 `429 rate_limit_error`。

## 观测日志

网关应输出结构化 access log。chat completions 请求至少包含：

- `request_id`
- `method`
- `path`
- `status`
- `latency_ms`
- `external_model`
- `provider`
- `upstream_model`
- `stream`
- `error_type`
- `error_code`

日志不得记录 `Authorization` header、上游 API key、完整 prompt 或完整 completion。

日志中的 `path` 字段只保留已知路由。未知路由统一归一化为 `/__unknown__`，避免异常路径进入日志索引造成高基数。

日志格式和级别可配置：

```json
{
  "log": {
    "format": "json",
    "level": "info"
  }
}
```

支持的 `format`：

- `text`
- `json`

支持的 `level`：

- `debug`
- `info`
- `warn`
- `error`

## 配置自检

网关支持不启动 HTTP server 的配置自检模式：

```bash
open-ai-gateway check-config
```

或：

```bash
GATEWAY_CHECK_CONFIG=1 open-ai-gateway
```

自检会加载配置、应用默认值、执行配置校验，并输出 provider/model 摘要和 warning。输出不得包含 API key 明文。

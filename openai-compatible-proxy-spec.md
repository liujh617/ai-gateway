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

流式响应：

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
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
| 鉴权失败 | `401` | `authentication_error` |
| 模型不存在或无权限 | `404` | `invalid_request_error` |
| provider 超时 | `504` | `server_error` |
| provider 返回限流 | `429` | `rate_limit_error` |
| provider 内部错误 | `502` | `server_error` |

## `GET /v1/models`

返回网关对当前调用方可见的模型列表。

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

暂不支持的高级字段可以先解析保留，但不得导致服务 panic。

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

- 每个事件以空行结束。
- handler 必须在写入 chunk 后 flush。
- request context 取消时必须关闭上游流。
- 上游流异常时，如果响应头尚未发送，返回 JSON 错误；如果已经开始 SSE，则发送兼容的错误 chunk 或直接结束连接，并记录日志。

## 模型映射

客户端只感知对外模型名。配置示例：

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

## 超时

建议第一阶段默认值：

- 普通请求总超时：60 秒
- 流式请求首 token 超时：60 秒
- 流式请求最大持续时间：10 分钟
- 上游连接超时：10 秒

所有超时应可配置。


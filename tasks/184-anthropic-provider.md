# Task 184 - Anthropic Provider + 格式转换层

## 目标

新增 Anthropic Provider，支持 OpenAI Chat Completions ↔ Anthropic Messages API 格式转换，使现有 OpenAI 兼容客户端（包括 Claude Code）可以通过网关访问 Anthropic 模型。

## 架构

```
Client (OpenAI format)    Gateway                     Upstream (Anthropic format)
  |                          |                            |
  |-- Chat Completions ---->|                            |
  |  (Bearer key, gpt-like) |-- Auth + Routing           |
  |                          |-- OpenAI → Anthropic 转换   |
  |                          |-- Messages API req ------->|
  |                          |<- Anthropic SSE/JSON ------|
  |                          |-- Anthropic → OpenAI 转换   |
  |<-- OpenAI SSE/JSON ------|                            |
```

## 转换映射

### 请求转换（OpenAI → Anthropic）

| OpenAI 字段 | Anthropic 字段 | 说明 |
|---|---|---|
| `model` | `model` | 通过路由映射到 upstream_model |
| `messages` | `messages` | 角色+内容转换 |
| `stream` | `stream` | 直接透传 |
| `temperature` | `temperature` | 直接透传 |
| `top_p` | `top_p` | 直接透传 |
| `max_tokens` | `max_tokens` | 直接透传 |
| `stop` | `stop_sequences` | 数组化 |
| `system` role msg | `system` (顶层) | 提取首条 system 消息 |
| `tools` | `tools` | 格式转换 |
| `tool_choice` | `tool_choice` | 格式转换 |

### 响应转换（Anthropic → OpenAI）

| Anthropic 字段 | OpenAI 字段 | 说明 |
|---|---|---|
| `id` | `id` | `msg_xxx` → `chatcmpl-xxx` |
| `model` | `model` | 对外模型名 |
| `usage` | `usage` | input_tokens/output_tokens |
| `stop_reason` | `choices[0].finish_reason` | end_turn→stop, max_tokens→length, tool_use→tool_calls |
| `content[].text` | `choices[0].message.content` | 文本拼接 |
| `content[].tool_use` | `choices[0].message.tool_calls` | 工具调用转换 |

### 流式转换

| Anthropic SSE | OpenAI SSE |
|---|---|
| `message_start` | 首个 `chat.completion.chunk` + `role` |
| `content_block_start(text)` | delta 首块（含 content 前缀） |
| `content_block_delta(text_delta)` | `choices[0].delta.content` |
| `content_block_start(tool_use)` | `choices[0].delta.tool_calls[0]` (含 id/name) |
| `content_block_delta(input_json_delta)` | `choices[0].delta.tool_calls[0].function.arguments` |
| `message_delta(usage)` | 末块 `usage` |
| `message_stop` | `[DONE]` |

### 错误转换

| Anthropic Error | OpenAI Error |
|---|---|
| `{"type":"error","error":{"type":"...","message":"..."}}` | `{"error":{"type":"...","message":"...","code":"..."}}` |

HTTP 状态码直接透传。

## 新建文件

```
internal/provider/anthropic/
  anthropic.go        # Provider 实现
  convert.go          # 格式转换函数
  anthropic_test.go   # 测试
  convert_test.go     # 转换测试
```

## 修改文件

```
internal/config/config.go        # 新增 "anthropic" provider type
schema/config.schema.json        # Schema 更新
cmd/gateway/main.go              # Provider 工厂
```

## 配置示例

```json
{
  "providers": {
    "claude": {
      "type": "anthropic",
      "base_url": "https://api.anthropic.com/v1",
      "api_key": "sk-ant-xxx"
    }
  },
  "models": {
    "claude-sonnet-5": {
      "provider": "claude",
      "upstream_model": "claude-sonnet-5-20251001",
      "capabilities": ["chat"]
    }
  }
}
```

## Provider 实现要点

### anthropic.go

```go
type Provider struct {
    baseURL  string
    apiKey   string
    client   *http.Client
}
```

- 认证：`x-api-key` header + `anthropic-version: 2023-06-01`
- 请求构造：`POST /v1/messages`
- 流式：`POST /v1/messages` + `Accept: text/event-stream`（但 SSE 格式不同）
- 注意：Anthropic 的流式格式不是标准 SSE，需要特殊处理

### convert.go

```go
func openaiToAnthropic(req compat.ChatCompletionRequest) anthropicRequest
func anthropicToOpenAI(resp anthropicResponse, model string) *compat.ChatCompletionResponse
func anthropicStreamToOpenAIChunk(event anthropicStreamEvent) *compat.ChatCompletionChunk
func anthropicErrorToOpenAI(errResp anthropicErrorResp) *compat.Error
```

## Anthropic Stream 格式

Anthropic 使用自己的流式格式（不是 SSE），每条事件以 `event:` 开头：

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_xxx",...}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}

event: message_stop
data: {"type":"message_stop"}
```

需要自定义流解析器（无法复用现有的 SSE 解析器）。

## 实施步骤

1. 创建 `internal/provider/anthropic/` 包
2. 实现 `convert.go`（请求+响应+流式+错误转换）
3. 实现 `anthropic.go`（Provider）
4. 实现 Anthropic SSE 流解析
5. 在 `config.go` 注册 `"anthropic"` provider type
6. 在 `main.go` 工厂中创建 Anthropic provider
7. 单元测试（convert_test.go + anthropic_test.go）
8. 集成测试 + `make verify`

## 验证

- `go test ./...`
- 转换 round-trip 测试
- 流式格式解析测试

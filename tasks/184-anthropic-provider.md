# Task 184 - Anthropic Provider + 格式转换层

## 背景

见 spec `openai-compatible-proxy-spec.md` Anthropic Provider 章节。

## 实施步骤

### Step 1：Anthropic 数据类型

`internal/provider/anthropic/types.go`：
- `MessageRequest`：顶层请求（model, messages, system, stream, max_tokens, temperature, top_p, stop_sequences, tools, tool_choice）
- `Message`：role + content blocks 数组
- `ContentBlock`：{type, text} | {type, id, name, input} | {type, tool_use_id, content}
- `MessageResponse`：id, model, role, content[], stop_reason, usage
- `MessageStreamEvent`：type, message, content_block, delta, usage
- `Tool`：name, description, input_schema
- `ErrorResponse`：type, error{type, message}

### Step 2：格式转换

`internal/provider/anthropic/convert.go`：

**请求转换**（重点）：
```go
func openaiToAnthropic(req compat.ChatCompletionRequest) MessageRequest
```
- 遍历 messages：第一条 system → MessageRequest.System
- user msg → ContentBlock{type:"text", text: content}
- assistant msg → ContentBlock{type:"text", text: content} + tool_use blocks
- tool msg → ContentBlock{type:"tool_result", tool_use_id, content}
- tools → Anthropic Tool 数组
- tool_choice 格式转换

**响应转换**：
```go
func anthropicToOpenAI(resp MessageResponse, model string) *compat.ChatCompletionResponse
```
- content blocks → choices[0].message.content + tool_calls
- stop_reason → finish_reason 映射
- usage 透传

**流式转换**：
```go
func anthropicStreamToOpenAIChunk(event MessageStreamEvent, model string) *compat.ChatCompletionChunk
```
- message_start → null（占位，实际数据在后续事件中）
- content_block_start(text) → 首次 delta 含 role
- content_block_delta(text_delta) → delta.content
- content_block_start(tool_use) → delta.tool_calls[0]{id, name}
- content_block_delta(input_json_delta) → delta.tool_calls[0].function.arguments
- message_stop → [DONE]

**错误转换**：
```go
func anthropicErrorToOpenAI(body []byte, statusCode int) *compat.Error
```

### Step 3：流式解析器

`internal/provider/anthropic/stream.go`：

Anthropic 流式格式每行以 `event:` 开头：
```
event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}
```

解析器：
```go
type MessageStream struct { reader io.Reader }
func NewMessageStream(r io.Reader) *MessageStream
func (s *MessageStream) Next(ctx context.Context) (*MessageStreamEvent, error)
```

### Step 4：Provider 实现

`internal/provider/anthropic/anthropic.go`：
- New(baseURL, apiKey string, timeout time.Duration)
- CreateChatCompletion → POST /v1/messages → 请求转换 + 响应转换
- StreamChatCompletion → POST /v1/messages + stream:true → 流式读取 + 事件转换
- 认证：x-api-key + anthropic-version headers
- 其他 Provider 接口方法返回 not implemented

### Step 5：注册 Provider

- `internal/config/config.go`：添加 `"anthropic"` case
- `cmd/gateway/main.go`：工厂创建 AnthropicProvider
- `schema/config.schema.json`：enum 增加 `"anthropic"`

### Step 6：测试

- `convert_test.go`：请求/响应/流式/错误 round-trip 测试
- `anthropic_test.go`：Provider CreateChatCompletion/StreamChatCompletion 测试
- 集成测试用 httptest 模拟 Anthropic API

### Step 7：验证

- `go test ./...`
- `make verify`

## 关键决策

1. **Anthropic 流式解析**：自实现（非标准 SSE），使用 bufio.Scanner
2. **Content 聚合**：多个 text block 拼接为单个字符串
3. **工具调用**：tool_use block → OpenAI tool_calls 数组，带 index 保持顺序
4. **只实现核心的 CreateChatCompletion + StreamChatCompletion**，其他 Provider 方法返回 `errors.New("not supported")`

# Responses API Function Tools 设计

## 背景

Task 157 已为 `POST /v1/responses` 提供无状态纯文本兼容，包括字符串与消息输入、`instructions`、非流式 output message 和 typed SSE 文本事件。下一步增加客户端自定义 function tools，使 agent 客户端能完成“模型请求调用函数、应用执行、回传结果、模型生成最终文本”的无状态闭环。

Responses API 与 Chat Completions 的 function schema、tool choice、输入 Item、输出 Item 和 streaming 事件结构不同。本设计继续在 compat 层转换，复用现有 chat provider pipeline，不扩展 provider 接口。

参考：

- [OpenAI Function Calling](https://developers.openai.com/api/docs/guides/function-calling)
- [Responses create reference](https://developers.openai.com/api/reference/resources/responses/methods/create)
- [Responses streaming events](https://developers.openai.com/api/reference/resources/responses/streaming-events)

## 目标

- 支持 Responses `tools` 中的 JSON Schema function tools。
- 支持 `tool_choice` 的 `auto`、`none`、`required` 和指定 function。
- 支持 `parallel_tool_calls` 和一次 response 中的多个 function calls。
- 支持无状态 input 中的 `function_call` 与字符串 `function_call_output` Item。
- 将 Chat Completions tool calls 转换为 Responses `function_call` output Items。
- 支持 function arguments typed SSE 事件和多个交错调用。
- 保持 text-only Responses 行为不回归。
- 对不支持的 tool 类型和无效关联稳定返回 `400 invalid_request_error`。

## 非目标

- 不执行客户端函数；函数仍由调用方执行。
- 不支持 `previous_response_id`、Conversations 或服务端 response 存储。
- 不支持 built-in tools、custom tools、namespace、allowed tools、tool search 或 MCP tools。
- 不支持 reasoning Items、encrypted reasoning 或 reasoning summaries。
- 不支持图片、文件或其他多模态 function output。
- 不在网关实现完整 JSON Schema validator 或 Responses schema normalization。
- 不增加 provider 专属 Responses 方法。

## 方案比较

### 方案一：compat 层双向转换（采用）

把 Responses function definitions、tool choice 和 Items 转换为现有 Chat Completions request，再把 chat tool calls 转回 Responses Items/events。

优点是所有现有 provider 自动获得能力，并继续复用 fallback、熔断、timeout、usage 和审计。缺点是只支持两种协议都能表达的 function tool 子集，必须严格拒绝其他工具。

### 方案二：只支持单个 function call

限制 `parallel_tool_calls: false`，遇到多个调用时返回错误。实现略简单，但不符合官方建议客户端应处理多个调用，也会降低 agent 兼容性，因此不采用。

### 方案三：provider 原生 Responses tools

扩展 provider 接口并原样透传 Responses。能力上限更高，但 Azure/OpenAI-compatible 上游支持不一致，且会扩大 fallback 和配置语义，因此不在本任务采用。

## 架构

```text
Responses tools / function Items
  -> compat 严格解析、关联校验和请求转换
  -> Chat Completions tools / tool_choice / messages
  -> router / fallback / circuit breaker / provider
  -> chat content / tool_calls / delta.tool_calls
  -> Responses message / function_call Items 和 typed SSE
```

职责边界：

- `internal/compat` 定义 function tool、input Item、output Item、chat tool-call 辅助类型及纯转换逻辑。
- `internal/api/responses.go` 负责 HTTP、provider 调用、stream 聚合、事件顺序、审计和取消。
- `internal/provider` 接口保持不变。
- OpenAI-compatible、Azure 和 fake provider 继续传输 Chat Completions JSON；不得加入 Responses 专属分支。

## Function Tool 请求契约

支持的工具形状：

```json
{
  "type": "function",
  "name": "get_weather",
  "description": "Get current weather",
  "parameters": {
    "type": "object",
    "properties": {
      "location": {"type": "string"}
    },
    "required": ["location"],
    "additionalProperties": false
  },
  "strict": true
}
```

校验规则：

- `type` 必须为 `function`。
- `name` 必填、不得包含首尾空白，同一请求中不得重复。
- `description` 可选；提供时必须是字符串。
- `parameters` 必填且 JSON 顶层必须为 object；内部 schema 交给上游校验。
- `strict` 可显式为 `true` 或 `false`；省略时网关补为 `true`。
- tools 数组提供时不得为空。

转换后的 Chat Completions tool 使用外部 `type: "function"` 和嵌套 `function` object。`strict` 始终成为显式布尔值。

## Tool Choice 与并行调用

支持：

- `"auto"`
- `"none"`
- `"required"`
- `{"type":"function","name":"get_weather"}`

指定 function 必须存在于同一请求的 tools 列表。`tool_choice` 在没有 tools 时只能省略或为 `none`；其他形式返回 400。

`parallel_tool_calls` 接受布尔值并转发给 Chat Completions。省略时不强行覆盖 provider 默认。网关必须能处理零个、一个或多个并行 tool calls；不因客户端省略该字段而限制为单调用。

`allowed_tools`、built-in tool choice 和其他 object 形状稳定返回 400。

## 无状态 Input Item 转换

除现有文本 message Item 外，input 数组新增两种类型。

先前 function call：

```json
{
  "type": "function_call",
  "id": "fc_...",
  "call_id": "call_...",
  "name": "get_weather",
  "arguments": "{\"location\":\"Paris\"}",
  "status": "completed"
}
```

函数结果：

```json
{
  "type": "function_call_output",
  "call_id": "call_...",
  "output": "{\"temperature\":25}"
}
```

关联规则：

- `call_id` 必填、不得包含首尾空白，在 function calls 中不得重复。
- 每个 output 的 `call_id` 必须匹配同一 input 数组中更早出现的 function call。
- 同一 call 最多出现一个 output。
- function call 的 `name` 和 `arguments` 必填；arguments 必须是编码合法 JSON value 的字符串。
- function call 的 `status` 省略或为 `completed`；其他值拒绝。
- output 只支持字符串。
- Responses Item `id` 不用于关联；转换到 chat 时使用 `call_id` 作为 tool call ID。

转换到 Chat Completions：连续 function call Items 可以合并为一条 assistant message 的 `tool_calls` 数组；每个 function output 转为 role `tool`、`tool_call_id` 为 call ID、content 为 output 字符串的 message。必须保持 input 的语义顺序，不能把 tool result 移到对应 call 之前。

## 非流式输出转换

继续只接受一个 chat choice。assistant message 的非空文本生成一个 Responses message Item。每个 chat `tool_calls[]` 生成一个 completed `function_call` Item：

```json
{
  "id": "fc_...",
  "type": "function_call",
  "status": "completed",
  "call_id": "call_...",
  "name": "get_weather",
  "arguments": "{\"location\":\"Paris\"}"
}
```

Responses Item ID 由网关生成；`call_id` 保留上游值。多个调用保持 chat tool_calls 数组顺序。若文本和 tool calls 同时存在，message Item 在前，function call Items 随后。

下列上游结果视为 provider conversion error：

- tool call ID、name 或 arguments 缺失。
- 重复 tool call ID。
- tool type 不是 function。
- arguments 不是编码合法 JSON value 的字符串。
- chat tool call index 或 choice 结构不受支持。

## 流式输出转换

文本继续使用 Task 157 的事件。每个 chat tool-call index 独立聚合，首次出现时分配 Responses output index 和 `fc_...` Item ID。

function call 生命周期：

1. `response.output_item.added`
2. 零个或多个 `response.function_call_arguments.delta`
3. `response.function_call_arguments.done`
4. `response.output_item.done`

要求：

- arguments delta 按 chat tool-call index 独立累计。
- 不同调用的 delta 可以交错。
- 同一 index 的 ID、name 不得在后续 chunk 冲突。
- tool-call index 不得为负数或重复表示不同调用。
- 全 stream 共用严格递增的 `sequence_number`。
- output index 按 Item 首次出现顺序分配。
- 同一 chat chunk 同时包含 text 与 tool delta 时，先输出 text 事件，再按 tool-call index 升序输出 tool 事件。
- EOF 时每个 arguments 累计值必须是合法 JSON，否则发送 typed `error`。
- 正常完成时先结束仍打开的文本 Item，再按 output index 完成各 function Item，最终 `response.completed.output` 包含全部 completed Items。

流开始后的结构错误发送 typed `error` 并关闭上游 stream；可 fallback provider 错误继续影响 provider health，但开始输出后不切换 provider。

## 错误处理

下列客户端错误返回稳定 `400 invalid_request_error`，并使用稳定字段路径：

- 非 function tool、空/重复 tool name、非法 parameters、strict 类型错误。
- 不支持或引用不存在 function 的 tool choice。
- parallel_tool_calls 类型错误。
- function call/output 缺字段、重复 call ID、output 先于 call、重复 output。
- 非字符串 output、非法 arguments JSON。

上游 tool call 结构不合法映射为稳定 gateway/provider error，内部细节写日志。已有鉴权、模型权限、fallback、熔断、timeout 和限流错误保持不变。

## 可观测性与审计

- audit 记录原始 tools、function call/output Items、非流式 function Items 和所有 function typed SSE events。
- 不在普通 access log 中记录完整 tool schema、arguments 或 output。
- usage/cost metrics 继续使用 provider usage，不因 function Item 数量重复计数。
- fallback、circuit 和 HTTP metrics 继续使用 `/v1/responses` path。
- 不新增以 function name 为 label 的 metrics，避免高基数和潜在敏感信息。

## 测试策略

至少覆盖：

- tools schema 转换，以及 strict 省略、true、false。
- 四类 tool choice 和所有拒绝形状。
- parallel_tool_calls 省略、true、false。
- 单个和多个 function calls。
- function call + output 的无状态第二轮转换。
- call ID 唯一性、顺序、关联和重复 output。
- 纯文本 input/output/stream 回归。
- 非流式 text-only、tool-only、text + tools。
- 非流式非法上游 tool calls。
- 流式单调用、多调用、交错 arguments、同 chunk 混合 text/tool delta。
- 流式冲突 ID/name/index、非法结束 arguments、typed error 和 stream close。
- fallback、熔断、timeout、取消、audit、usage/cost metrics。
- OpenAI-compatible、Azure 和 fake provider 通过现有接口工作。

新增无需真实凭据的 `make smoke-responses-tools`。fake provider/test upstream 必须完成两轮：第一轮输出 function call；脚本构造 function call + output input；第二轮获得最终文本。该 smoke 接入 `release-check`，使用独立端口且不访问外网。

文档提供官方 OpenAI Python 或 JavaScript SDK 的可选本地验证命令；标准验证不依赖 SDK。

最终验证在 WSL `Ubuntu-24.04` 执行：

```bash
make verify
make smoke-responses-tools
make release-check
```

## 文档同步

- 新增 `tasks/158-responses-function-tools.md`。
- 更新 `openai-compatible-proxy-spec.md`。
- 更新 `architecture/overview.md`。
- 更新 `README.md`、`docs/local-verification.md`、`docs/ci.md` 和 `CHANGELOG.md`。

## 验收标准

- 官方 SDK 可声明 function tools、接收一个或多个 function calls、回传字符串 outputs 并获得最终文本。
- non-stream 与 streaming 均输出符合 Responses 语义的 function Items/events。
- call ID 在无状态两轮调用中准确关联。
- 不支持工具和无效结构稳定返回 400，不静默降级。
- provider 接口不变，现有 provider 和纯文本 Responses 行为不回归。
- audit、metrics、fallback、熔断、timeout 和取消符合现有约定。
- WSL 完整验证与新增 smoke 通过。

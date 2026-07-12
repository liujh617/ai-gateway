# Responses API 最小文本兼容设计

## 背景

`open-ai-gateway` 0.1.2 已支持 OpenAI-compatible chat completions、streaming chat、embeddings、静态模型路由、fallback、provider circuit breaker、鉴权、限流、审计和可观测性，并可连接 OpenAI-compatible 与 Azure OpenAI 上游。

OpenAI 当前建议新项目优先使用 Responses API。Responses API 使用 typed Items 表示输入和输出，并使用独立的 typed SSE 事件；它不是 Chat Completions 的简单字段别名。为了扩大官方 SDK 和新客户端对本项目的兼容性，下一阶段先实现一个无状态、纯文本的 `/v1/responses` 最小闭环，同时继续复用现有 provider 能力。

参考：

- [Migrate to the Responses API](https://developers.openai.com/api/docs/guides/migrate-to-responses)
- [Responses API reference](https://developers.openai.com/api/reference/resources/responses)
- [Responses streaming events](https://developers.openai.com/api/reference/resources/responses/streaming-events)

## 目标

- 新增 `POST /v1/responses`。
- 支持 `input` 字符串和纯文本消息数组。
- 支持顶层 `instructions`。
- 支持非流式文本响应。
- 支持 Responses typed SSE 文本流。
- 通过兼容层转换复用现有 Chat Completions provider、路由、fallback、熔断、超时和 metrics。
- 对未实现能力返回稳定的 OpenAI-compatible `400 invalid_request_error`，不静默忽略。
- 保持 Go 标准库优先，不新增运行时依赖。

## 非目标

- 不支持 `previous_response_id`、Conversations API 或其他服务端状态。
- 不支持 `store: true`；网关不保存 response。
- 不支持 function tools、built-in tools、MCP、tool calls 或 tool outputs。
- 不支持图片、音频、文件或其他多模态输入输出。
- 不支持 reasoning Items、encrypted reasoning 或 reasoning summaries。
- 不支持 background responses、WebSocket 或 response cancellation API。
- 不把 provider 接口扩展为原生 Responses API 接口。

## 方案比较

### 方案一：网关兼容层转换（采用）

在 API/compat 层把 Responses 文本请求转换为现有 `ChatCompletionRequest`，调用现有 provider 接口，再把 chat response 或 chunk 转换为 Responses 对象或 typed SSE 事件。

优点：所有现有 provider 自动获得最小 Responses 支持；复用 fallback、熔断、timeout 和 usage metrics；变更边界小。缺点：只能诚实支持 Chat Completions 可以表达的能力，必须严格拒绝状态、工具和多模态字段。

### 方案二：只支持字符串 input

只接受 `input: "..."`，以最小代码量提供 endpoint。

优点：实现最小。缺点：无法覆盖从 Chat Completions 迁移而来的消息数组，实际 SDK 兼容价值有限，因此不采用。

### 方案三：provider 原生 Responses 透传

扩展 provider 接口，让支持 Responses API 的上游直接接收和返回完整契约。

优点：长期能力上限最高。缺点：OpenAI-compatible、Azure OpenAI 和其他上游支持不一致，会扩大 provider、router 和配置面，不适合作为首个增量，因此暂不采用。

## 架构与边界

请求流程：

```text
POST /v1/responses
  -> Responses JSON 解析与严格校验
  -> 转换为 ChatCompletionRequest
  -> 现有 router / fallback / circuit breaker / provider
  -> 转换为 Response 对象或 Responses typed SSE
```

组件职责：

- `internal/compat`：定义首期 Responses 请求、响应、Item、usage 和 stream event 类型，以及纯转换和校验逻辑。
- `internal/api/responses.go`：处理 HTTP、模型解析、审计、超时、fallback 调用、SSE 写入和客户端取消。
- `internal/router`：继续使用模型的 `chat` capability，不新增 `responses` capability。
- `internal/provider`：接口保持不变；继续调用 `CreateChatCompletion` 和 `StreamChatCompletion`。
- `internal/routes`：注册 `POST /v1/responses`，使其进入现有鉴权、限流、method handling 和 HTTP metrics。

Responses handler 可以抽取并复用 chat handler 的 provider attempt/fallback helper，但不得把 Responses 格式逻辑写入 provider adapter，也不得在 handler 中加入 provider 专属分支。

## 请求契约

首期请求字段：

- `model`：必填、非空字符串，按现有 external model 路由。
- `input`：必填；可以是非空字符串，或非空的纯文本消息数组。
- `instructions`：可选非空字符串。
- `stream`：可选布尔值，默认 `false`。
- `store`：可省略或显式为 `false`；`true` 返回 400。

消息数组支持 `user`、`assistant`、`system` 和 `developer` role。message content 接受非空字符串，或仅包含文本 part 的非空数组；文本 part 必须使用 `type: "input_text"` 和非空 `text`。图片、文件、音频、tool item、混合 content part 和其他 Item 类型一律拒绝。

转换规则：

- `instructions` 转换为首条 `developer` chat message。
- 字符串 `input` 转换为一条 `user` chat message。
- 消息数组按原顺序转换，保留支持的 role 和纯文本内容。
- external model 在 handler/router 层解析，provider 收到现有逻辑映射后的 upstream model。
- `stream` 控制调用现有非流式或流式 provider 方法。

首期采用严格字段策略。除上述字段外，任何顶层字段均返回 `400 invalid_request_error`；不支持的嵌套字段或 Item 类型也返回同类错误。错误消息包含稳定字段路径，例如 `unsupported field: previous_response_id`。这样可避免请求成功但关键选项未生效。

## 非流式响应契约

成功响应返回 OpenAI Responses `response` 对象的核心字段：

- 网关生成稳定格式的 `resp_...` ID。
- `object` 为 `response`。
- `created_at` 为 Unix 秒时间戳。
- `status` 为 `completed`。
- `model` 为客户端请求的 external model。
- `output` 包含一个 assistant message Item。
- message Item 使用独立的 `msg_...` ID、`type: "message"`、`status: "completed"`、`role: "assistant"`。
- content 包含一个 `type: "output_text"` part，带完整 `text` 和空 `annotations`。
- `usage.input_tokens`、`usage.output_tokens`、`usage.total_tokens` 分别映射 chat usage 的 prompt、completion 和 total token。

为保证官方 SDK 可反序列化，官方 schema 中需要稳定出现的其余字段使用明确的 `null`、空数组或与首期语义一致的默认值。`output_text` 是 SDK helper 的派生结果，不作为网关自行增加的 JSON 字段。

如果上游返回零个或多个 choices、tool call、非文本 content 或其他无法无损映射的结构，网关返回转换错误，不选择其中一项或伪造文本结果。

## 流式响应契约

流式响应使用 `Content-Type: text/event-stream`。每个事件同时包含 SSE `event:` 和 `data:`：

```text
event: response.output_text.delta
data: {"type":"response.output_text.delta",...}

```

正常文本流按下列顺序输出：

1. `response.created`
2. `response.in_progress`
3. `response.output_item.added`
4. `response.content_part.added`
5. 一个或多个 `response.output_text.delta`
6. `response.output_text.done`
7. `response.content_part.done`
8. `response.output_item.done`
9. `response.completed`

Responses stream 不发送 Chat Completions 的 `[DONE]` sentinel。同一请求内 response ID、message ID、`output_index` 和 `content_index` 必须稳定；`sequence_number` 从固定起点严格递增。每个上游文本 delta 映射为一个 `response.output_text.delta`，网关累计完整文本供各 `done` 事件使用。若上游提供 usage，则映射到最终 `response.completed`。

流建立前发生错误时返回普通 OpenAI-compatible JSON error。响应头已经提交后发生错误时发送 Responses `error` typed event，然后关闭连接。客户端取消必须取消 context 并关闭上游 stream。流式读取阶段失败继续沿用现有规则：不切换 provider，但会影响后续 provider health 判断。

## 错误处理

- 非 JSON content type、无效 JSON、trailing JSON、body 超限、字段类型错误：`400 invalid_request_error`。
- 缺少或为空的 `model`/`input`、非法 role/content：`400 invalid_request_error`。
- `previous_response_id`、`tools`、图片等所有未支持字段或能力：`400 invalid_request_error`。
- `store: true`：`400 invalid_request_error`。
- 模型不存在、client model allowlist 和缺少 `chat` capability：复用现有稳定错误。
- provider 429、5xx、timeout、fallback 和 circuit breaker：复用现有语义。
- 无法从 chat 结果无损转换到首期 Responses 文本契约：返回稳定 gateway/upstream error，内部细节只写日志。

错误响应继续使用项目已有的 OpenAI-compatible error envelope，不引入第二套错误格式。

## 可观测性与审计

- HTTP、fallback、circuit open、provider health、token 和 cost metrics 继续复用现有实现。
- 请求级 metrics 的 `path` 标签使用 `/v1/responses`，不能错误归入 `/v1/chat/completions`。
- client、external model、provider 和 upstream model 维度保持现有语义。
- audit 记录原始 Responses 请求、最终 Responses 响应或 typed stream events；默认仍关闭，并遵守现有敏感数据警告和轮转配置。
- 日志不得输出 Authorization header、provider key 或默认记录完整 prompt/completion。

## 测试策略

新增单元与 API 集成测试，至少覆盖：

- 字符串 input、消息数组和 `instructions` 的精确转换。
- 标准非流式 response、ID、output Item 和 usage 映射。
- typed SSE 完整事件顺序、`event:`/`data:`、稳定 ID、索引、递增 sequence number 和累计文本。
- 无效 JSON、trailing JSON、body 超限、缺失字段、非法 role/content。
- 每类明确不支持的顶层能力，以及代表性的嵌套非文本 Item。
- `store` 缺省、`false` 和被拒绝的 `true`。
- 鉴权失败、模型不存在、模型 capability 和 client model allowlist。
- provider error、timeout、fallback、circuit breaker 和流式建连失败。
- 流开始后的上游错误、context cancellation 和上游 stream close。
- audit 与 `/v1/responses` 路径下的 HTTP、token、cost、fallback 和 circuit metrics。
- OpenAI-compatible、Azure OpenAI 和 fake provider 通过同一现有 provider 接口工作，不增加 provider 专属 Responses 分支。

增加不依赖真实凭据的 `make smoke-responses` 服务级 smoke，并用官方 OpenAI Python 或 JavaScript SDK 验证 `responses.create()` 与 streaming 能消费网关输出。SDK 只用于可选的开发验证，不成为 Go 服务的运行时依赖；标准 `make verify` 和 `make smoke-responses` 不依赖外部网络或预装 SDK。文档提供单独、可重复执行的 SDK 验证命令。

最终验证在 WSL `Ubuntu-24.04` 执行：

```bash
make verify
make smoke-responses
```

`make smoke-responses` 接入 `release-check`，使发布验证持续覆盖 Responses 非流式和流式文本契约。

## 文档与任务同步

实现外部 API 契约时同步更新：

- `openai-compatible-proxy-spec.md`
- `architecture/overview.md`
- `README.md`
- `docs/local-verification.md`
- `CHANGELOG.md`
- `tasks/157-responses-api-text.md`

文档必须明确区分“已支持字段”和“稳定拒绝字段”，并说明 Responses 是经 chat provider 转换实现的无状态纯文本子集。

## 发布边界

本设计是 `v0.2.0` 的第一个增量。Task 157 只交付无状态纯文本闭环；状态存储、function tools、原生 provider Responses、多模态和其他 Responses 子资源必须分别设计和实施，不混入本任务。

## 验收标准

- 官方 OpenAI SDK 可通过网关调用最小 `responses.create()` 文本用例并读取输出文本。
- 官方 OpenAI SDK 可消费网关的 typed streaming 文本事件。
- 字符串 input、消息数组和 `instructions` 均被准确转换。
- 未支持能力稳定返回 `400 invalid_request_error`，不存在静默忽略。
- 所有现有 provider 无需新增 Responses 专属方法即可工作。
- fallback、熔断、超时、取消、audit 和 metrics 行为与现有 chat 路径一致，并正确归因到 `/v1/responses`。
- 文档、task、兼容契约和实现一致。
- WSL `Ubuntu-24.04` 中的完整验证通过。

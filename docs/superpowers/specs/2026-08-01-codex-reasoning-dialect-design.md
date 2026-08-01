# Codex Responses 与多模型 Reasoning Dialect 设计

## 背景

`open-ai-gateway` 已经能够把 `/v1/responses` 的文本和 function tools 转换为 OpenAI-compatible Chat Completions，但当前实现不支持 reasoning Items。DeepSeek thinking mode 在 assistant 发生工具调用后，要求后续请求完整回传该 assistant 消息的 `reasoning_content`；网关当前会丢弃该字段，导致 Codex 在工具结果回传后收到：

```text
The `reasoning_content` in the thinking mode must be passed back to the API.
```

Codex 当前使用 Responses API 的无状态调用方式：`store:false`，不依赖 `previous_response_id`，并在后续请求中重放先前返回的 Items。OpenAI Responses API 使用 `reasoning.encrypted_content` 在这种无状态模式中携带不透明 reasoning 状态，而 DeepSeek Chat Completions 使用 assistant message 上的明文 `reasoning_content`。网关需要在两种状态模型之间建立安全、可扩展的协议桥。

参考：

- [OpenAI Encrypted Reasoning Items](https://developers.openai.com/cookbook/examples/responses_api/reasoning_items#encrypted-reasoning-items)
- [OpenAI Responses statefulness](https://developers.openai.com/api/docs/guides/migrate-to-responses#4-decide-when-to-use-statefulness)
- [DeepSeek thinking mode](https://api-docs.deepseek.com/zh-cn/guides/thinking_mode)

## 目标

- 让 Codex 通过 `/v1/responses`、`stream:true`、`store:false` 使用 DeepSeek thinking mode 和 function tools。
- 在工具调用续接中完整、安全地恢复 DeepSeek `reasoning_content`。
- 支持共享密钥的网关实例交替处理同一个 Codex task，并支持网关重启后的续接。
- 引入厂商无关的 conversation IR 和显式 dialect adapter，使后续 Kimi、GLM 等模型能够独立适配。
- 保持普通 OpenAI-compatible、Chat Completions、现有 Responses 文本/function tools 和 response store 行为兼容。
- 保持现有审计配置、事件格式和存储行为不变。

## 非目标

- 本期不实现 Kimi 或 GLM 的真实 adapter，只提供可扩展接口和测试契约基础。
- 不改造现有 audit recorder、JSONL 格式、明文存储、轮转或 inspect 行为。
- 不支持 built-in tools、图片、音频、文件、MCP wire translation 或其他新 Responses Item。
- 不展示 DeepSeek raw reasoning，也不把 raw reasoning 伪装成 reasoning summary。
- 不引入数据库、分布式缓存、全局 replay blacklist 或 envelope 一次性消费语义。
- 不实现运行时密钥热加载；密钥轮换通过配置更新和滚动重启生效。
- 不进行与本功能无关的锁、性能或大范围结构重构。

## 核心决策

采用“规范化 conversation IR + provider dialect adapter”方案：

```text
Codex Responses Items
        ↓
Responses wire codec
        ↓
Gateway conversation IR
        ↓
Provider dialect adapter
        ↓
OpenAI-compatible HTTP transport
        ↓
DeepSeek Chat Completions
```

Codex Responses wire format、规范化对话语义、reasoning envelope、厂商差异和 HTTP 传输分别位于独立边界。不得在 Responses handler 中根据 provider name 或 base URL 编写 DeepSeek 条件分支。

不采用以下方案：

- 每个 provider 各自实现完整 Responses bridge：会重复 validation、SSE、工具关联、错误和 envelope 逻辑。
- 配置驱动字段映射引擎：thinking/tool-call 是有状态协议，不是字段重命名；通用映射语言会扩大安全和测试面。
- 仅使用进程内 reasoning cache：无法满足 `store:false`、重启和多实例续接。
- Base64 或明文 envelope：无法提供保密性、完整性和租户绑定。

## 组件边界

### `internal/compat`

负责解析和生成 OpenAI/Codex Responses wire format：

- Response request、response object 和 typed SSE event。
- `message`、`reasoning`、`function_call`、`function_call_output` 和 `additional_tools` 的 wire 类型。
- wire 类型与 conversation IR 之间的转换。

该包不感知 DeepSeek、Kimi、GLM 或 provider URL。

### `internal/conversation`

定义厂商无关、HTTP 无关的对话模型。首期只表达：

- `Message`：system、user、assistant 文本。
- `Reasoning`：网关内存中的 reasoning 明文、assistant content、关联 tool call IDs 和 route binding。
- `FunctionCall`：call ID、name、arguments。
- `FunctionOutput`：call ID、output。
- `Opaque`：仅保存 dialect 明确允许但 IR 不解释的元数据。

IR 必须保留 Item 顺序和 reasoning/tool-call 分组。类型使用包内封闭接口或等价 tagged union，包外只能通过公开构造和校验函数创建有效值。

该包负责：

- call ID 唯一性。
- function call/output 关联。
- reasoning envelope call ID 集合与当前 function-call turn 的完全匹配。
- 重复 output、未知 call ID、跨 turn 配对和顺序错误检测。

### `internal/reasoningenvelope`

负责 versioned envelope 的签发和打开：

- AES-256-GCM 加密与认证。
- active/previous key 选择。
- TTL、audience、client、model 和 route binding 校验。
- 密文与明文大小限制。
- 对外稳定错误分类。

该包不依赖 HTTP、router、provider 或 audit。

### `internal/provider/dialect`

定义 dialect 注册、能力和转换接口。概念接口为：

```go
type Dialect interface {
	Name() string
	Capabilities() Capabilities
	BuildChatRequest(conversation.Turn, RequestOptions) (compat.ChatCompletionRequest, error)
	ParseChatResponse(*compat.ChatCompletionResponse) (conversation.Turn, error)
	NewStreamDecoder() StreamDecoder
}
```

最终签名可以按现有 provider 接口适配，但必须保持以下边界：

- dialect 处理语义转换。
- `internal/provider/openai` 处理 OpenAI-compatible HTTP、JSON 和 SSE 传输。
- API handler 只做编排，不处理厂商字段。

首期注册：

- `openai-compatible`
- `deepseek`

后续可以独立注册 `kimi`、`glm`。

### `internal/provider/deepseek`

实现 DeepSeek dialect：

- `reasoning_content` 输入、输出和 streaming delta。
- thinking 参数及 reasoning effort 映射。
- assistant tool-call content 非 null 约束。
- thinking mode 下 `tool_choice` 兼容规则。
- developer role 映射。
- Responses 专属参数过滤。

### `internal/api`、router 和 config

- API 层编排鉴权、client allowlist、限制、路由、dialect、provider 调用、SSE、metrics、fallback、response store 和现有 audit。
- model route 显式选择 dialect，不根据 base URL 猜测。
- 含 active tool reasoning 的 continuation 固定到 envelope 绑定的 provider、upstream model 和 dialect。

## Reasoning Envelope

### Wire 形状

DeepSeek 返回 reasoning 后，网关向 Codex 返回 Responses reasoning item：

```json
{
  "id": "rs_...",
  "type": "reasoning",
  "summary": [],
  "encrypted_content": "gwre1.<key-id>.<base64url-payload>"
}
```

前缀 `gwre1` 表示网关 reasoning envelope v1。key ID 只用于选择解密密钥，不包含密钥材料。

### 明文 payload

加密前 payload 至少包含：

- envelope schema version。
- 随机 envelope ID。
- dialect、provider、external model 和 upstream model。
- gateway client name。
- issued-at 和 expires-at。
- 完整 `reasoning_content`。
- assistant 可见 content。
- 本 reasoning turn 关联的全部 tool call IDs。

payload 使用严格 JSON schema 解码；未知关键版本、缺失必填字段、重复 call ID 或超过限制均拒绝。

### 密码学约束

- 使用 Go 标准库 AES-256-GCM。
- key 环境变量解码后必须恰好 32 字节。
- 每次签发使用 `crypto/rand` 生成独立 nonce。
- AAD 绑定 envelope version、deployment audience、gateway client 和 external model。
- active key 可签发和解密；previous keys 仅可解密。
- key ID 在 active/previous keys 中必须唯一。
- 格式错误、未知 key、认证失败、过期或 binding 不匹配对客户端统一归类为 `invalid reasoning item`。
- 不在错误、普通日志或密码学诊断中输出密钥、nonce、明文或完整 envelope。

纯无状态方案不提供严格的一次性使用保证。client binding、鉴权、短期 TTL、请求限制和现有限流用于降低 replay 风险；全局 replay 防护留给后续需要共享状态的设计。

### 配置

```json
{
  "reasoning_envelope": {
    "enabled": true,
    "audience": "ai-gateway-prod",
    "ttl_seconds": 3600,
    "max_envelope_bytes": 1048576,
    "max_plaintext_bytes": 524288,
    "max_items_per_request": 256,
    "max_tool_calls_per_turn": 64,
    "active_key": {
      "id": "reasoning-2026-01",
      "key_env": "GATEWAY_REASONING_KEY"
    },
    "previous_keys": [
      {
        "id": "reasoning-2025-12",
        "key_env": "GATEWAY_REASONING_PREVIOUS_KEY"
      }
    ]
  }
}
```

安全默认值使用上述数值。`enabled=false` 时不加载 envelope keys。

任一主 route 或 fallback 启用 `reasoning_replay` 时：

- `reasoning_envelope.enabled` 必须为 `true`。
- audience、TTL、限制、active key ID 和 active key env 必须有效。
- active/previous key env 必须存在且可解析。
- 配置错误导致启动失败，不能静默关闭 reasoning 或降级为明文。

`check-config` 只报告 enabled、audience、TTL、限制、active key ID、active key 是否可用和 previous key 数量，不输出 key env 的值或密钥。

## Route 和 Dialect 配置

model route 增加显式字段：

```json
{
  "models": {
    "gpt-5.6-sol": {
      "provider": "deepseek",
      "upstream_model": "deepseek-v4-pro",
      "dialect": "deepseek",
      "reasoning_replay": true,
      "capabilities": ["chat"]
    }
  }
}
```

fallback route 同样可以声明 `dialect` 和 `reasoning_replay`。

规则：

- `dialect` 省略时默认 `openai-compatible`，保持旧配置兼容。
- dialect 必须存在于注册表。
- `reasoning_replay=true` 要求 dialect 声明支持 reasoning replay。
- 请求 input 中只要仍重放任一带 tool call IDs 的 reasoning envelope，就视为该 provider reasoning history 仍然有效，只允许选择完全匹配的 provider、upstream model 和 dialect。
- 固定 route 不可用时返回现有 provider unavailable、timeout 或 upstream error，不切换到不匹配 fallback，也不丢弃 reasoning。
- 只有新 task 或客户端明确不再携带任何带 tool call IDs 的 reasoning envelope 时，才恢复正常主路由和 fallback 行为。

## Codex 请求转换

Responses input 必须按数组原始顺序转换：

- `message` → IR Message。
- `reasoning` → 打开 envelope 后生成 IR Reasoning。
- `function_call` → IR FunctionCall。
- `function_call_output` → IR FunctionOutput。
- `additional_tools` → 合并到本次工具定义。
- 其他 item → `400 invalid_request_error`。

reasoning/function 关联规则：

- envelope 的 call ID 集合必须与对应 function-call turn 完全一致。
- 每个 function output 必须引用此前出现且尚未完成的 call ID。
- 不允许重复 call ID、重复 output、跨 reasoning turn 配对或丢失 call。
- 不得把 function calls 在第二遍统一追加到消息尾部；IR 和最终 Chat messages 必须保持 turn 顺序。

旧网关输出的历史不包含 reasoning item。thinking tool-call 历史缺少 reasoning 时，网关必须在上游调用前返回：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "reasoning item is required for this tool call",
    "param": "input"
  }
}
```

升级前已运行的 Codex task 需要新建 task；网关不得伪造或忽略缺失 reasoning。

## DeepSeek 请求转换

DeepSeek dialect 执行：

- `developer` role 映射为 `system`。
- Codex reasoning effort：`low`、`medium`、`high` 映射为 `high`，`xhigh` 映射为 `max`。
- 显式发送 `thinking: {"type":"enabled"}`。
- thinking mode 不向 DeepSeek 发送 `tool_choice`。
- assistant tool-call message 的 `content` 必须非 null；优先使用 envelope 中的原始 content，否则使用空字符串。
- assistant message 写入完整 `reasoning_content` 和 `tool_calls`。
- `include`、`store`、`text`、`client_metadata`、`prompt_cache_key` 和顶层 Responses `reasoning` 等专属字段不透传给 DeepSeek。
- 只发送 DeepSeek dialect 明确允许或明确映射的请求参数。

`openai-compatible` dialect 保持当前普通 Chat/Responses 行为，不自动启用 thinking 或 envelope。

## DeepSeek 响应转换

### 非流式

固定 Responses output 顺序：

1. reasoning item。
2. 非空 assistant message。
3. 一个或多个 function call items。

当上游返回 `reasoning_content` 时：

- reasoning 只在内存中用于签发 envelope。
- Responses reasoning item 的 summary 为空。
- 不将 raw reasoning 写入 output text 或 summary。
- reasoning item 绑定该 assistant turn 的全部 tool call IDs。

thinking mode 返回 tool calls 但缺少非空 `reasoning_content` 时，视为不完整 provider response，返回 `502 provider returned an incomplete reasoning tool call`，不向 Codex 返回无法续接的 function call。

### Streaming

- DeepSeek `reasoning_content` delta 只在内存中累计。
- 普通 assistant text 继续使用现有 `response.output_text.delta` 事件。
- tool call ID、name 和 arguments 完整累计。
- 为了绑定全部并行 call IDs，首期在上游 turn 完成后签发 reasoning envelope，再提交 reasoning 和 function-call Items。
- 先发送 reasoning 的 `response.output_item.added` 和 `response.output_item.done`。
- 再按稳定顺序发送每个 function call 的既有 typed argument/output-item events。
- `response.completed.output` 的 Item 顺序必须与事件顺序一致。
- 不发送 `response.reasoning_summary_text.delta`，因为 DeepSeek raw reasoning 不是用户可见 summary。
- stream 在 envelope 签发前失败时，只发送 Responses `error`，不签发、不保存残缺 reasoning，并关闭上游 stream。

首期允许 reasoning/tool-call Items 在 upstream turn 完成后才交付，不以 tool-argument 最低延迟为目标。

## Response Store

无状态 encrypted reasoning 是 Codex 默认兼容路径，但现有 response store 继续可选：

- `store:false` 不需要服务端保存 reasoning。
- `store:true` 或 `previous_response_id` 可以保存规范化 transcript；保存的 assistant tool-call message必须包含恢复后的 reasoning 语义，深拷贝时不得丢失。
- response store 不替代 envelope；Codex 手工重放 Items 和 `previous_response_id` 两种方式均须工作。
- store TTL、容量和 client 隔离规则保持不变。

## 错误契约

| 条件 | HTTP / event | 稳定错误 |
|---|---:|---|
| envelope 格式、key、认证或过期错误 | 400 | `invalid reasoning item` |
| client/model/route binding 不匹配 | 400 | `invalid reasoning item` |
| envelope 与 function calls 不匹配 | 400 | `reasoning item does not match function calls` |
| thinking tool history 缺少 reasoning | 400 | `reasoning item is required for this tool call` |
| 上游 tool call 缺少 reasoning | 502 | `provider returned an incomplete reasoning tool call` |
| 固定 route 不可用 | 现有 provider error | 保持现有映射 |
| streaming 中途失败 | Responses SSE `error` | 不返回残缺 envelope |

所有 envelope 打开失败共享粗粒度外部错误，避免形成 token/key 探测 oracle。内部日志只能记录错误分类、key ID 和 envelope ID 等非秘密元数据。

## 管控顺序与限制

本期不新增完整 policy engine，但 reasoning 请求必须经过现有管控并按以下顺序执行：

1. gateway 鉴权。
2. gateway client model allowlist。
3. 请求体和 input item 限制。
4. envelope 格式、大小、TTL 和 binding 校验。
5. conversation 顺序及 call ID 校验。
6. route 解析或固定 route 检查。
7. provider 调用。
8. 现有限流、usage、cost、health、metrics 和 audit。

新增的 envelope、plaintext、item 数和每 turn tool-call 数量限制必须在分配大对象或调用上游前执行。

## 审计边界

本期保持项目当前审计实现不变：

- `audit.enabled/path/max_file_bytes` 配置不变。
- JSONL event schema、完整 API body、stream chunk 和 error 记录行为不变。
- 不增加 metadata、encrypted audit、双侧 wire audit、retention 或防篡改。

已接受的限制：当前显式启用的完整明文 audit 会记录 Codex 请求和网关响应，因此 reasoning envelope 密文可能进入审计文件。它不应记录 envelope 解密密钥或网关内部 reasoning 明文；现有 Responses 转换后的 event 仍不是原始 upstream wire audit。

## 测试策略

所有生产行为按 TDD 实现，先运行并观察针对缺失行为的测试失败。

### Envelope 单元测试

- AES-GCM round trip 和随机 nonce。
- active/previous key 解密及轮换。
- tamper、unknown key、过期、audience 错误。
- client/model/provider/upstream binding。
- ciphertext/plaintext 限制。
- 对外错误不泄露内部细节。

### Conversation 和 compat 测试

- reasoning → function call → function output 保序。
- 多轮和并行 call ID 关联。
- output-before-call、重复 call/output、缺失 reasoning 和跨 turn 关联拒绝。
- Responses 专属字段不会进入 DeepSeek request。
- 旧 tool history 返回稳定缺失 reasoning 错误。

### DeepSeek dialect 测试

- reasoning effort、thinking 参数和 role 映射。
- 移除 `tool_choice`。
- assistant content 非 null。
- 完整恢复 reasoning、content 和 tool calls。
- 非流式 reasoning/message/function output 顺序。
- streaming delta 聚合和 envelope 签发。
- provider 不完整响应和 context cancellation。

### API 和服务测试

- `httptest` DeepSeek upstream 完成至少两轮 thinking + tool call。
- `stream:true`、`store:false` 且无 `previous_response_id`。
- 单个、连续和并行 tools。
- 两个共享 key 的 Server 实例交替续接。
- 不同 key、client、model 或 route 拒绝续接。
- 固定 route 不跨 provider fallback。
- response store 模式仍可工作。
- 原有 Chat Completions、普通 Responses、SSE、取消和错误契约不回归。

### Smoke 和最终验证

- 增加无需真实凭据的 DeepSeek dialect/Codex Responses smoke。
- 可选真实 DeepSeek smoke 在没有 `DEEPSEEK_API_KEY` 时明确跳过，在有 key 时覆盖 thinking + function tool，不只验证文本。
- 使用最小脱敏 fixture，不提交现有真实 audit 内容。
- 最终在 WSL `Ubuntu-24.04` 执行 `make verify` 和新增 smoke。

## 文档和 ADR

实现时同步更新：

- `openai-compatible-proxy-spec.md`
- `architecture/overview.md`
- `schema/config.schema.json` 和示例配置
- `README.md`
- `docs/local-verification.md`
- `docs/ci.md`、`docs/release.md`
- `CHANGELOG.md`
- 对应 task 文档

新增 ADR，记录 conversation IR、显式 dialect、无状态 encrypted reasoning envelope、route pinning 和不采用服务端 reasoning cache 的决策。

## 迁移

- 旧配置未声明 `dialect` 时继续使用 `openai-compatible`。
- DeepSeek Codex route 需要显式配置 `dialect:"deepseek"`、`reasoning_replay:true`、全局 reasoning envelope 和 key 环境变量。
- 旧 Codex task 因历史没有 reasoning item，升级后无法继续 thinking tool call，需要新建 task。
- 新 task 的 envelope 在 TTL 内可跨实例和重启续接，前提是 audience、route 和密钥集合一致。

## 验收清单

- Codex 能通过网关对 DeepSeek 完成 thinking + function tool 闭环。
- 不再由缺失 `reasoning_content` 触发 DeepSeek 400。
- `store:false`、无 `previous_response_id` 正常工作。
- 共享 key 的实例可以交替续接，重启不丢失 envelope 能力。
- 无效、过期、跨 client/model/route envelope 在上游调用前失败。
- raw reasoning 不作为可见 Responses text 或 summary 返回。
- 普通 OpenAI-compatible、Chat Completions、response store 和现有 audit 不回归。
- Kimi、GLM 后续可以通过新增 dialect 实现接入，而无需修改 Codex Responses handler。

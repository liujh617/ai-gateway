# Codex Reasoning Dialect Bridge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Codex 通过 `/v1/responses`、`stream:true`、`store:false` 安全使用 DeepSeek thinking mode 和 function tools，并为后续 Kimi、GLM dialect 保留清晰扩展点。

**Architecture:** Responses wire codec 先把 Codex Items 规范化为 `internal/conversation` IR，再由 route 选择的 `internal/provider/dialect` 转换为 Chat Completions。DeepSeek reasoning 通过 `internal/reasoningenvelope` 使用 AES-256-GCM 封装为 `reasoning.encrypted_content`，后续请求解密后恢复 `reasoning_content`；带 tool reasoning 的请求固定到 envelope 绑定的 route。

**Tech Stack:** Go 1.22 标准库（`crypto/aes`、`crypto/cipher`、`crypto/rand`、`encoding/base64`、`encoding/json`、`net/http`）、现有 `net/http` API/provider/router/responsestore、WSL `Ubuntu-24.04`。

## Global Constraints

- 实现依据：`docs/superpowers/specs/2026-08-01-codex-reasoning-dialect-design.md`。
- 只实现 DeepSeek 真实 dialect；Kimi、GLM 本期只共享接口，不写未经真实协议验证的 adapter。
- 不改变现有 audit 配置、JSONL event schema、明文存储、轮转和 inspect 行为。
- 不引入第三方 Go 依赖；优先并默认只使用标准库。
- 不在 handler 中写 provider 专属逻辑；不在 provider adapter 中写 gateway client 鉴权逻辑。
- raw reasoning 只在请求内存、可选 response store 和 AES-GCM 加解密边界内出现；不得作为 Responses 可见 text/summary 或普通日志字段输出。
- reasoning envelope key 只从环境变量读取，配置错误在启动阶段失败。
- 保持旧配置默认 `dialect: openai-compatible`，普通 Chat Completions 和其他 endpoint 不回归。
- 所有行为变更按 RED → GREEN → REFACTOR 执行；没有先观察到预期失败，不写对应生产代码。
- 标准验证环境固定为 WSL `Ubuntu-24.04`；完整验证使用 `make verify`。
- 实施前使用 `superpowers:using-git-worktrees` 创建 `E:\code\ai-gateway\.worktrees\codex-reasoning-dialect`，分支名 `codex/codex-reasoning-dialect`。当前主工作区未提交文件不得带入、覆盖或清理。

---

### Task 1: Reasoning 与 Dialect 配置及 Router 元数据

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/examples_test.go`
- Modify: `internal/router/model_router.go`
- Modify: `internal/router/model_router_test.go`
- Modify: `cmd/gateway/main.go`
- Modify: `cmd/gateway/main_test.go`
- Modify: `schema/config.schema.json`
- Modify: `config.example.json`

**Interfaces:**
- Produces: `config.ReasoningEnvelopeConfig`、`config.ReasoningEnvelopeKeyConfig`。
- Produces: `ModelConfig.Dialect string`、`ModelConfig.ReasoningReplay bool`，fallback 上相同字段。
- Produces: `router.ProviderRoute.Dialect string`、`router.ProviderRoute.ReasoningReplay bool`。
- Produces: `func (r ModelRoute) MatchAttempt(providerName, upstreamModel, dialect string) (ProviderRoute, bool)`。
- Default dialect: `openai-compatible`。

- [ ] **Step 1: 写配置失败测试**

在 `internal/config/config_test.go` 增加表驱动测试，至少覆盖：

```go
func TestLoadReasoningEnvelopeConfig(t *testing.T) {
	t.Setenv("ACTIVE_REASONING_KEY", base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	path := writeConfig(t, validConfigWith(`
		"reasoning_envelope": {
			"enabled": true,
			"audience": "gateway-prod",
			"ttl_seconds": 3600,
			"max_envelope_bytes": 1048576,
			"max_plaintext_bytes": 524288,
			"max_items_per_request": 256,
			"max_tool_calls_per_turn": 64,
			"active_key": {"id":"active","key_env":"ACTIVE_REASONING_KEY"}
		},
	`))
	cfg, err := config.Load(path)
	if err != nil { t.Fatal(err) }
	if !cfg.ReasoningEnvelope.Enabled || cfg.ReasoningEnvelope.ActiveKey.ID != "active" {
		t.Fatalf("reasoning envelope = %#v", cfg.ReasoningEnvelope)
	}
}
```

拒绝用例必须包含：启用 replay 但 envelope disabled、空 audience、非正 TTL、非正限制、重复 key ID、缺少 key env、非 base64url key、解码后不是 32 字节、未知 dialect、`reasoning_replay=true` 配 `openai-compatible`。

- [ ] **Step 2: 写 router 失败测试**

在 `internal/router/model_router_test.go` 验证主 route/fallback copy 和 `Attempts()` 保留 dialect/replay，并验证精确匹配：

```go
func TestModelRouteMatchAttemptIncludesDialect(t *testing.T) {
	route := ModelRoute{
		ExternalModel: "codex-model",
		ProviderName: "primary", UpstreamModel: "deepseek-v4-pro",
		Dialect: "deepseek", ReasoningReplay: true,
	}
	got, ok := route.MatchAttempt("primary", "deepseek-v4-pro", "deepseek")
	if !ok || !got.ReasoningReplay { t.Fatalf("match = %#v, %t", got, ok) }
	if _, ok := route.MatchAttempt("primary", "deepseek-v4-pro", "glm"); ok {
		t.Fatal("mismatched dialect matched")
	}
}
```

- [ ] **Step 3: 运行测试并确认 RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "go test ./internal/config ./internal/router ./cmd/gateway -run 'ReasoningEnvelope|Dialect|MatchAttempt' -count=1"
```

Expected: FAIL，原因是配置字段、校验或 route 元数据尚不存在。

- [ ] **Step 4: 实现配置类型和默认值**

在 `internal/config/config.go` 增加：

```go
type ReasoningEnvelopeKeyConfig struct {
	ID     string `json:"id"`
	KeyEnv string `json:"key_env"`
}

type ReasoningEnvelopeConfig struct {
	Enabled             bool                         `json:"enabled"`
	Audience            string                       `json:"audience"`
	TTLSeconds          int                          `json:"ttl_seconds"`
	MaxEnvelopeBytes    int                          `json:"max_envelope_bytes"`
	MaxPlaintextBytes   int                          `json:"max_plaintext_bytes"`
	MaxItemsPerRequest  int                          `json:"max_items_per_request"`
	MaxToolCallsPerTurn int                          `json:"max_tool_calls_per_turn"`
	ActiveKey           ReasoningEnvelopeKeyConfig   `json:"active_key"`
	PreviousKeys        []ReasoningEnvelopeKeyConfig `json:"previous_keys"`
}
```

`Config` 增加 `ReasoningEnvelope`；model 和 fallback 增加 `Dialect`、`ReasoningReplay`。默认限制严格使用 spec 数值。只有 replay route 存在时才强制 envelope 与 key env；环境变量用 `base64.RawURLEncoding` 解码并验证 32 字节。

- [ ] **Step 5: 实现 route 元数据和构建 wiring**

给 `router.ModelRoute` 和 `router.ProviderRoute` 增加 dialect/replay 字段，更新 `copy()`、`Attempts()`、`buildRouter()`、`fallbackRoutes()`。实现精确 `MatchAttempt`；不得按 URL 推断 dialect。

- [ ] **Step 6: 更新 schema、example 和 check report**

Schema 对 dialect 使用 `openai-compatible|deepseek` enum。`CheckReport` 只报告 envelope enabled、audience、TTL、限制、active key ID/是否设置、previous key 数量；不能输出 key 内容。

- [ ] **Step 7: 运行 focused tests 并确认 GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "gofmt -w internal/config internal/router cmd/gateway && go test ./internal/config ./internal/router ./cmd/gateway -count=1"
```

Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add internal/config internal/router cmd/gateway schema/config.schema.json config.example.json
git commit -m "feat: configure reasoning dialect routes"
```

---

### Task 2: AES-GCM Reasoning Envelope

**Files:**
- Create: `internal/reasoningenvelope/envelope.go`
- Create: `internal/reasoningenvelope/envelope_test.go`

**Interfaces:**
- Produces: `reasoningenvelope.Codec`。
- Produces: `reasoningenvelope.RouteBinding`、`Binding`、`Payload`。
- Produces: `func New(Config) (*Codec, error)`。
- Produces: `func (c *Codec) Seal(Payload, Binding) (string, error)`。
- Produces: `func (c *Codec) Open(string, Binding) (Payload, error)`。
- Produces: sentinel `ErrInvalid`，所有外部无效 token 都满足 `errors.Is(err, ErrInvalid)`。

- [ ] **Step 1: 写 round-trip 和随机 nonce 失败测试**

```go
func TestCodecSealOpenRoundTrip(t *testing.T) {
	codec := newTestCodec(t, time.Unix(100, 0))
	binding := Binding{Audience:"prod", Client:"alpha", ExternalModel:"gpt-5.6-sol"}
	want := Payload{
		EnvelopeID:"env_1",
		Route:RouteBinding{Dialect:"deepseek", Provider:"deepseek", UpstreamModel:"deepseek-v4-pro"},
		ReasoningContent:"private cot",
		AssistantContent:"", CallIDs:[]string{"call_1"},
	}
	token, err := codec.Seal(want, binding)
	if err != nil { t.Fatal(err) }
	got, err := codec.Open(token, binding)
	if err != nil { t.Fatal(err) }
	if diff := cmpPayload(want, got); diff != "" { t.Fatal(diff) }
}
```

第二个测试连续 Seal 同一 payload，断言 token 不相等且均以 `gwre1.active.` 开头。

- [ ] **Step 2: 写安全失败测试**

覆盖：一个字节篡改、未知 key ID、错误 prefix、错误 base64、错误 audience/client/model、过期、未来 issued-at、重复 call ID、密文过大、明文过大。所有 token 校验失败只断言 `errors.Is(err, ErrInvalid)`，不得依赖密码学细节文本。

- [ ] **Step 3: 写轮换失败测试**

用 old codec 签发，再用 active+previous codec 打开；反向确认 previous key 不用于新签发。

- [ ] **Step 4: 运行并确认 RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "go test ./internal/reasoningenvelope -count=1"
```

Expected: FAIL，package/API 尚不存在。

- [ ] **Step 5: 实现最小 envelope codec**

核心类型固定为：

```go
type RouteBinding struct {
	Dialect, Provider, UpstreamModel string
}

type Binding struct {
	Audience, Client, ExternalModel string
}

type Payload struct {
	Version          int          `json:"v"`
	EnvelopeID       string       `json:"id"`
	Route            RouteBinding `json:"route"`
	Client           string       `json:"client"`
	ExternalModel    string       `json:"external_model"`
	IssuedAt         int64        `json:"iat"`
	ExpiresAt        int64        `json:"exp"`
	ReasoningContent string       `json:"reasoning_content"`
	AssistantContent string       `json:"assistant_content"`
	CallIDs          []string     `json:"call_ids"`
}
```

`Config` 注入 `Now func() time.Time` 和 `Rand io.Reader` 以便测试。AAD 使用长度前缀编码绑定 version/audience/client/model，避免字符串拼接歧义。token 只包含 prefix、key ID、base64url(nonce+ciphertext)。

- [ ] **Step 6: 运行测试并确认 GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "gofmt -w internal/reasoningenvelope && go test ./internal/reasoningenvelope -count=1"
```

Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/reasoningenvelope
git commit -m "feat: add encrypted reasoning envelopes"
```

---

### Task 3: Conversation IR 与 Function 关联校验

**Files:**
- Create: `internal/conversation/types.go`
- Create: `internal/conversation/validate.go`
- Create: `internal/conversation/validate_test.go`

**Interfaces:**
- Produces: `conversation.Request`、`Turn`、封闭 `Item` 接口。
- Produces: `Message`、`Reasoning`、`FunctionCall`、`FunctionOutput`、`FunctionTool`、`Opaque`。
- Produces: `func ValidateRequest(Request, Limits) error`。
- Produces: `func (r Request) PinnedRoute() (conversation.RouteBinding, bool, error)`。

- [ ] **Step 1: 写合法顺序失败测试**

构造 `Reasoning(call_1,call_2) → FunctionCall(call_1) → FunctionCall(call_2) → FunctionOutput(call_1) → FunctionOutput(call_2)`，断言校验通过且 `PinnedRoute()` 返回 DeepSeek route。

- [ ] **Step 2: 写拒绝矩阵失败测试**

表驱动覆盖：重复 call ID、重复 output、未知 output、output-before-call、reasoning call ID 集合不完整、跨 reasoning turn 复用 call、不同 reasoning envelope 绑定不同 route、超过 item/tool-call/reasoning 字节限制。

```go
func TestValidateRequestRejectsOutputBeforeCall(t *testing.T) {
	req := Request{Turn: Turn{Items: []Item{
		FunctionOutput{CallID:"call_1", Output:json.RawMessage(`"x"`)},
		FunctionCall{CallID:"call_1", Name:"f", Arguments:"{}"},
	}}}
	if err := ValidateRequest(req, testLimits()); err == nil {
		t.Fatal("expected correlation error")
	}
}
```

- [ ] **Step 3: 运行并确认 RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "go test ./internal/conversation -count=1"
```

Expected: FAIL，IR 尚不存在。

- [ ] **Step 4: 实现封闭 IR**

`Request` 明确保存 model、stream、ordered turn、function tools、tool choice、reasoning effort 和需要由默认 dialect 兼容透传的 extra fields；`Item` 使用包内 `isConversationItem()` 封闭实现。`Reasoning` 保存 envelope ID、明文、assistant content、call IDs 和 `conversation.RouteBinding`；`FunctionCall` 保存 `ReasoningEnvelopeID`，使同一 call 明确归属 reasoning turn；`Opaque` 仅能由受信 wire codec 通过构造函数创建。所有构造复制 byte slice、RawMessage、map 和 string slice。

- [ ] **Step 5: 实现单遍校验**

按 Item 顺序维护 call ownership 和 completed 状态；禁止后补关联。`PinnedRoute()` 扫描所有带非空 call IDs 的 Reasoning：无此类 item 返回 false；存在时必须全部 route 相同。

- [ ] **Step 6: 运行测试并确认 GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "gofmt -w internal/conversation && go test ./internal/conversation -count=1"
```

Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/conversation
git commit -m "feat: add normalized conversation IR"
```

---

### Task 4: Provider Dialect 契约、Registry 与默认实现

**Files:**
- Create: `internal/provider/dialect/dialect.go`
- Create: `internal/provider/dialect/registry.go`
- Create: `internal/provider/dialect/openai.go`
- Create: `internal/provider/dialect/dialect_test.go`

**Interfaces:**
- Produces: `dialect.Dialect`、`Capabilities`、`Request`、`Response`、`StreamEvent`、`StreamDecoder`
- Produces: `dialect.Registry`，启动期注册，运行期只读
- Produces: `dialect.NewOpenAICompatible()`

- [ ] **Step 1: 写 registry 与能力声明失败测试**

验证空名称、重复注册、未知名称查找失败，以及默认 dialect 声明 `ReasoningReplay=false`。

- [ ] **Step 2: 写默认 request/response 转换失败测试**

构造包含 system/user/assistant、普通 function call/output、tools、tool choice、reasoning effort 和 unknown top-level fields 的 IR；断言 `openai-compatible` dialect 转换后语义不变，且不会生成 `reasoning_content`。

- [ ] **Step 3: 写默认 stream decoder 失败测试**

输入现有 OpenAI-compatible text delta、tool call delta、finish reason 和 usage chunks，断言输出规范事件能重建同一 assistant turn；损坏 tool arguments 或重复 finish 必须报错。

- [ ] **Step 4: 运行并确认 RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "go test ./internal/provider/dialect -count=1"
```

Expected: FAIL，dialect contract 尚不存在。

- [ ] **Step 5: 实现最小契约与 registry**

固定边界：

```go
type Capabilities struct {
	ReasoningReplay bool
}

type Request struct {
	Conversation  conversation.Request
	UpstreamModel string
}

type Response struct {
	Turn  conversation.Turn
	Usage *compat.Usage
}

type StreamEvent struct {
	TextDelta string
	Usage     *compat.Usage
}

type StreamDecoder interface {
	Push(compat.ChatCompletionChunk) ([]StreamEvent, error)
	Finish() (Response, error)
}

type Dialect interface {
	Name() string
	Capabilities() Capabilities
	BuildChatRequest(Request) (compat.ChatCompletionRequest, error)
	ParseChatResponse(compat.ChatCompletionResponse) (Response, error)
	NewStreamDecoder() StreamDecoder
}
```

如果现有 `compat` 类型名不同，先复用现有类型并同步修正本计划中的示例，不新增同义 DTO。Registry 拒绝重复名称，`Get` 对未知名称返回稳定错误。

- [ ] **Step 6: 实现 `openai-compatible` dialect**

把现有 Responses→Chat 的通用转换移入默认 dialect；保留未知字段透传和普通 tool call 关联。此步骤不得改变当前非 reasoning 请求的 wire 输出。

- [ ] **Step 7: 运行 focused tests 并确认 GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "gofmt -w internal/provider/dialect && go test ./internal/provider/dialect ./internal/compat -count=1"
```

Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add internal/provider/dialect internal/compat
git commit -m "feat: add provider dialect contract"
```

---

### Task 5: DeepSeek Thinking Dialect

**Files:**
- Create: `internal/provider/deepseek/dialect.go`
- Create: `internal/provider/deepseek/stream.go`
- Create: `internal/provider/deepseek/dialect_test.go`
- Create: `internal/provider/deepseek/stream_test.go`

**Interfaces:**
- Produces: `deepseek.NewDialect()` implementing `dialect.Dialect`
- Consumes: `conversation.Reasoning.Route` and `ReasoningEnvelopeID`
- Emits: canonical assistant `Reasoning` followed by owned `FunctionCall` items

- [ ] **Step 1: 写初始 thinking 请求失败测试**

断言 Codex `reasoning.effort` 映射到 DeepSeek thinking 开关/参数；`developer` message 按已批准设计降级为 `system`；Responses-only 字段不会发送上游；thinking 模式下不发送不受支持的 `tool_choice`。

- [ ] **Step 2: 写 tool continuation 失败测试**

输入解密后的：

```text
Reasoning(env_1, call_1) → FunctionCall(call_1) → FunctionOutput(call_1)
```

断言 DeepSeek messages 顺序严格为 assistant（`reasoning_content`、原 assistant content、原 tool_calls）后接 tool output；assistant content 为空时发送 `""` 而不是遗漏字段。

- [ ] **Step 3: 写关联和能力拒绝测试**

拒绝缺少 reasoning、envelope ID 不一致、call ID 集合不一致、输出无所有者、以及 route binding 与本次 attempt 不一致。错误在任何上游 HTTP 调用前产生。

- [ ] **Step 4: 写非流式响应解析失败测试**

模拟同时返回 `reasoning_content`、空 content、两个 tool calls 和 usage 的 DeepSeek 响应；断言生成一个 Reasoning item，后跟两个归属同一 envelope ID 的 FunctionCall。纯文本回答不得创建空 reasoning envelope。上游返回 tool calls 但 reasoning 为空时，必须返回可映射为 502 的 `provider returned an incomplete reasoning tool call`，不得下发 calls。

- [ ] **Step 5: 写流式 accumulator 失败测试**

覆盖 reasoning delta 跨 chunk、text delta、两个交错 tool-call index、arguments 分片、usage 尾块、finish；断言原始 reasoning 不出现在 `StreamEvent.TextDelta`，只在 `Finish()` 的规范 turn 中出现。覆盖重复 index/call ID、无 finish、损坏 arguments 和 cancellation 后不可继续 Push。

- [ ] **Step 6: 运行并确认 RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "go test ./internal/provider/deepseek -count=1"
```

Expected: FAIL，DeepSeek dialect 尚不存在。

- [ ] **Step 7: 实现请求映射和响应解析**

provider-specific JSON 仅通过现有 `compat.ChatMessage.Extra` 或集中定义的 DeepSeek wire DTO 表达；不得把 `reasoning_content` 字段判断放进 API handler。为新 reasoning turn 生成不可预测 envelope ID，测试通过注入 ID generator 固定结果。

- [ ] **Step 8: 实现 streaming state machine**

状态机只向 API 暴露安全的 text delta 和最终规范 turn；raw reasoning 只保存在请求生命周期 accumulator 中。`Finish()` 负责完整性校验，失败时不得生成可回放 envelope。

- [ ] **Step 9: 运行 focused tests 并确认 GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "gofmt -w internal/provider/deepseek && go test ./internal/provider/deepseek ./internal/provider/dialect -count=1"
```

Expected: PASS。

- [ ] **Step 10: 提交**

```bash
git add internal/provider/deepseek internal/provider/dialect
git commit -m "feat: add DeepSeek reasoning dialect"
```

---

### Task 6: Codex Responses Items 编解码

**Files:**
- Modify: `internal/compat/responses.go`
- Modify: `internal/compat/responses_test.go`
- Create: `internal/compat/responses_conversation.go`
- Create: `internal/compat/responses_conversation_test.go`

**Interfaces:**
- Produces: `func (r ResponseRequest) Conversation(OpenReasoning) (conversation.Request, *Error)`
- Produces: `func NewResponseEnvelopeFromTurn(ResponseRequest, conversation.Turn, *Usage, SealReasoning) (ResponseEnvelope, *Error)`
- Produces: request/response wire support for `reasoning.encrypted_content`
- Preserves: existing `ResponseRequest.ChatRequest()` during migration until all old callers are switched

- [ ] **Step 1: 写 Codex input 解码失败测试**

覆盖 string input、message item、function_call、function_call_output、reasoning item 与 `encrypted_content`。用 fake opener 返回规范 Reasoning，断言 wire 中伪造的 summary/raw fields 不会覆盖已认证 payload。

- [ ] **Step 2: 写 envelope 关联失败测试**

覆盖缺失 `encrypted_content`、打开失败、payload call IDs 与随后 function_call 不一致、function output 提前、多个 envelope route 不一致，以及旧 thinking tool history 完全缺少 reasoning item。最后一种必须返回 `reasoning item is required for this tool call`；其他 token 打开失败统一为稳定的 OpenAI-compatible 400，不暴露 key ID、AEAD 或解密细节。

- [ ] **Step 3: 写 Codex output 编码失败测试**

输入 `Reasoning → FunctionCall` turn，fake sealer 返回 `token_1`；断言 output 顺序为 reasoning item（含 `encrypted_content`）后跟 function_call，summary 不包含 raw reasoning。纯文本响应不调用 sealer。

- [ ] **Step 4: 写 wire 兼容失败测试**

验证 `store:false`、无 `previous_response_id`、`reasoning.effort`、未知顶层字段的现有行为不回归；旧的纯 message/tool Responses JSON golden 保持一致。

- [ ] **Step 5: 运行并确认 RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "go test ./internal/compat -run 'Response.*(Conversation|Reasoning|Encrypted|Tool)' -count=1"
```

Expected: FAIL，conversation codec 和 reasoning item 字段尚不存在。

- [ ] **Step 6: 实现 input codec**

`OpenReasoning` 由 API 传入闭包，以当前 audience、client identity、external model 作为 binding 打开 token。codec 只负责 wire↔IR，不自行选择 provider，不读取环境变量。

- [ ] **Step 7: 实现 output codec**

`SealReasoning` 仅接受完整并已校验的 Reasoning。输出 reasoning item 使用稳定 ID；`encrypted_content` 是唯一可回放私有载荷，`summary` 保持空数组或经过明确安全策略生成，本期不生成 reasoning 摘要。

- [ ] **Step 8: 运行 focused tests 并确认 GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "gofmt -w internal/compat && go test ./internal/compat -count=1"
```

Expected: PASS。

- [ ] **Step 9: 提交**

```bash
git add internal/compat
git commit -m "feat: add Codex reasoning item codec"
```

---

### Task 7: 非流式 Responses 执行、Route Pinning 与 Store

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/responses.go`
- Modify: `internal/api/responses_test.go`
- Modify: `internal/api/fallback.go`
- Create: `internal/api/fallback_test.go`
- Modify: `internal/responsestore/store.go`
- Modify: `internal/responsestore/store_test.go`

**Interfaces:**
- `api.Options` consumes immutable dialect registry and optional reasoning envelope codec
- Responses fallback execution selects dialect per `router.ProviderRoute`
- Response store persists normalized `conversation.Turn`/history instead of lossy chat messages

- [ ] **Step 1: 写普通 route dialect 选择失败测试**

构造 primary `openai-compatible`、fallback `deepseek`，分别让 primary 成功和失败；断言每个 attempt 只调用自己的 dialect，handler 中没有 provider-name 特判。

- [ ] **Step 2: 写 pinned route 失败测试**

携带绑定到 `deepseek/provider-a/model-a` 的 reasoning envelope，请求可用 routes 还包含 provider-b；断言只匹配完全相同的 provider/upstream model/dialect attempt。匹配不存在时返回 400/409 类稳定客户端错误，且零上游调用；不得静默 fallback。

- [ ] **Step 3: 写能力与配置防线失败测试**

当请求含 replay reasoning，但 route `ReasoningReplay=false`、dialect capability false 或 server 未配置 codec 时，断言请求在上游前失败。普通 Responses 请求仍可在无 codec 配置下工作。

- [ ] **Step 4: 写非流式端到端失败测试**

使用 fake DeepSeek upstream 执行两次：第一次返回 reasoning+tool call，网关返回 encrypted reasoning item；第二次把该 item、function_call 和 output 原样送回，断言上游收到必须回放的 `reasoning_content` 并完成文本回答。

- [ ] **Step 5: 写 response store 迁移失败测试**

验证 `store:true` / `previous_response_id` 仍可恢复完整 IR，reasoning 私有字段不会因 ChatMessage 降级而丢失；`store:false` 不写 store。既有 retrieve/delete 契约保持不变。

- [ ] **Step 6: 运行并确认 RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "go test ./internal/api ./internal/responsestore -run 'Responses|Pinned|Dialect|Reasoning' -count=1"
```

Expected: FAIL，API 尚未经过 dialect/IR 执行。

- [ ] **Step 7: 实现 attempt-aware fallback hook**

扩展通用 fallback executor，使 request preparation/response parsing 能接收当前 `ProviderRoute`，同时保持 Chat Completions、Embeddings 现有调用路径。API 只按 route 名称从 registry 获取 dialect；vendor JSON 仍封装在 dialect。

- [ ] **Step 8: 实现绑定、pinning 与非流式处理**

client binding 使用现有已认证 client identity 的稳定标识；不能使用 Authorization 原文。先解密和校验整个输入，再求 pinned route，再发起上游请求。成功响应在返回客户端前 sealing；sealing 失败返回内部错误且不下发不完整 tool turn。

- [ ] **Step 9: 把 response store 升级为 IR**

保存深拷贝后的规范 items；retrieve 仍返回原有 Responses envelope。仅调整内部存储表示，不改变 audit event 或审计写入时机。

- [ ] **Step 10: 运行 focused tests 并确认 GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "gofmt -w internal/api internal/responsestore && go test ./internal/api ./internal/responsestore -count=1"
```

Expected: PASS。

- [ ] **Step 11: 提交**

```bash
git add internal/api internal/responsestore
git commit -m "feat: execute Responses through dialect bridge"
```

---

### Task 8: Streaming Responses 安全状态机

**Files:**
- Modify: `internal/api/responses.go`
- Create: `internal/api/responses_stream_test.go`
- Modify: `internal/api/fallback.go`
- Modify: `internal/api/fallback_test.go`

**Interfaces:**
- Consumes: `dialect.StreamDecoder`
- Emits: existing Responses SSE lifecycle plus reasoning/function-call output items
- Guarantees: raw reasoning delta never appears in visible text or audit log

- [ ] **Step 1: 写完整 DeepSeek stream 失败测试**

fake upstream 依次发送 reasoning delta、tool call name/arguments 分片、usage 和 `[DONE]`；断言：

- `Content-Type: text/event-stream`
- 每个事件使用 `data:`
- 生命周期事件顺序合法
- reasoning output item 在 function_call item 之前完成
- reasoning item 含可打开的 `encrypted_content`
- 正常结束发送 `[DONE]`
- 完整 SSE body 不含 raw reasoning 明文

- [ ] **Step 2: 写普通文本 stream 回归测试**

断言 text delta 仍低延迟逐块下发，默认 `openai-compatible` dialect 的事件顺序和 usage 不回归。

- [ ] **Step 3: 写异常 stream 失败测试**

覆盖上游在 reasoning/tool call 中途断开、malformed chunk、缺失 finish、codec seal 失败。断言发送协议级 error/failed event，且不发送可被下轮接受的 reasoning envelope。

- [ ] **Step 4: 写 cancellation 失败测试**

取消客户端 context，断言上游 stream 被关闭、decoder 不再接收 chunk、没有 `[DONE]` 或完成 envelope，现有 cancellation metric/audit 行为不变。

- [ ] **Step 5: 运行并确认 RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "go test ./internal/api -run 'Responses.*Stream|Stream.*Reasoning|Cancellation' -count=1"
```

Expected: FAIL，Responses stream 尚未使用 dialect decoder 和终局 sealing。

- [ ] **Step 6: 实现安全 streaming 编排**

API 收到每个 chat chunk 后交给当前 dialect decoder；立即发送安全 text events，内部累积 reasoning/tool state。只有上游正常完成且 `Finish()` 校验成功后，才 seal reasoning 并发送相关 output-item 完成事件。

- [ ] **Step 7: 统一失败终止路径**

确保 response body 已开始后不会尝试写普通 JSON error；SSE 失败事件不包含 upstream body、raw reasoning、key ID 或密文解码细节。defer 关闭上游 body，context cancellation 优先终止读取。

- [ ] **Step 8: 运行 focused tests 并确认 GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "gofmt -w internal/api && go test ./internal/api -count=1"
```

Expected: PASS。

- [ ] **Step 9: 提交**

```bash
git add internal/api
git commit -m "feat: stream encrypted reasoning items"
```

---

### Task 9: 启动 Wiring 与跨实例无状态回放

**Files:**
- Modify: `cmd/gateway/main.go`
- Modify: `cmd/gateway/main_test.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/responses_test.go`

**Interfaces:**
- Startup builds one immutable dialect registry and optional `reasoningenvelope.Codec`
- Startup fails when any configured replay route cannot satisfy its capabilities
- Multiple instances sharing key/audience/config can replay each other's envelopes

- [ ] **Step 1: 写启动强校验失败测试**

表驱动覆盖：replay route 但 envelope disabled、缺 key env、错误 key 长度、未知 dialect、dialect 不声明 replay capability、DeepSeek replay 配置正确。错误信息必须定位配置路径，但不得打印 key value。

- [ ] **Step 2: 写旧配置兼容失败测试**

加载不含 `reasoning_envelope` 和 `dialect` 的现有示例，断言成功启动、所有 routes 使用 `openai-compatible`、普通 Chat/Responses 行为不变。

- [ ] **Step 3: 写双实例失败测试**

创建 server A/B，各自使用独立 memory response store、相同 active key/audience 和 route config。A 产出 envelope，B 在 `store:false`、无 `previous_response_id` 请求中成功回放。再验证：

- B 使用不同 key 时在上游前拒绝
- B audience 不同时拒绝
- client identity 不同时拒绝
- external model 不同时拒绝
- active key 已轮换但旧 key 在 `previous_keys` 时成功

- [ ] **Step 4: 运行并确认 RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "go test ./cmd/gateway ./internal/api -run 'Startup|LegacyConfig|Stateless|CrossInstance|KeyRotation' -count=1"
```

Expected: FAIL，最终 startup wiring 和跨实例路径尚未完成。

- [ ] **Step 5: 实现 startup construction**

在 `cmd/gateway` 集中注册 `openai-compatible` 和 `deepseek` dialect，从已经校验过的配置解析 key material 并构造 codec，再注入 server。所有 key byte slice 在可行范围内避免多余复制；check/report 仍只显示 key ID 与是否设置。

- [ ] **Step 6: 实现最终 capability preflight**

启动时遍历 primary/fallback routes：`ReasoningReplay=true` 必须同时满足 envelope enabled、codec available、dialect registered 且 capability true。任何一项失败都拒绝启动，不延迟到首个请求。

- [ ] **Step 7: 运行 focused tests 并确认 GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "gofmt -w cmd/gateway internal/api && go test ./cmd/gateway ./internal/api -count=1"
```

Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add cmd/gateway internal/api
git commit -m "feat: wire stateless reasoning replay"
```

---

### Task 10: 配置契约、架构、ADR、任务记录与 Smoke

**Files:**
- Modify: `openai-compatible-proxy-spec.md`
- Modify: `architecture/overview.md`
- Create: `docs/adr/0002-codex-reasoning-dialect-bridge.md`
- Create: `tasks/185-codex-reasoning-dialect-bridge.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/local-verification.md`
- Modify: `docs/ci.md`
- Modify: `docs/release.md`
- Modify: `scripts/smoke-deepseek.sh`
- Modify: `Makefile`

**Documentation must state:**
- Supported Codex mode: `/v1/responses`, `store:false`, no `previous_response_id`, streaming and tools
- Envelope threat/binding model, key rotation and mandatory startup policy
- Route pinning behavior and intentional no-fallback rule during replay
- DeepSeek is the only verified reasoning dialect in this release; Kimi/GLM are future adapters
- Audit behavior is unchanged by this feature

- [ ] **Step 1: 写 smoke 失败断言**

扩展现有 fake/real DeepSeek smoke，使其执行完整 `tool call → encrypted reasoning replay → final answer`。脚本在缺少 `encrypted_content`、泄露 raw reasoning、第二轮未回放 `reasoning_content` 或未收到 `[DONE]` 时失败。

- [ ] **Step 2: 运行 smoke 并确认 RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "make smoke-deepseek"
```

Expected: FAIL，新 smoke 场景尚未由 Make target 完整支持，或功能断言尚未满足。

- [ ] **Step 3: 更新外部兼容契约和架构**

在兼容 spec 中列出接受/返回的 Responses reasoning item 形态、稳定错误、limits 和 streaming 事件；架构图中加入 wire codec→IR→dialect→provider 与 envelope boundary。明确 raw reasoning 不作为可见文本，但现有 audit 的整体模式不在本期重构。

- [ ] **Step 4: 写 ADR 与 task 验收记录**

ADR 记录选择 conversation IR + dialect adapter、放弃 provider-name handler branching 与仅进程内缓存的原因。Task 文档引用设计 spec、implementation plan、测试矩阵和回滚边界。

- [ ] **Step 5: 更新 README、CHANGELOG 和本地验证文档**

给出仅含占位 key 的最小配置，key 生成命令不得回显真实生产 key。说明轮换时先把旧 active 放入 `previous_keys`，部署所有实例，再切换 active；移除旧 key 要等待 TTL 窗口。

- [ ] **Step 6: 让 smoke GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "make smoke-deepseek"
```

Expected: PASS；如真实 DeepSeek smoke 需要凭据，无凭据环境必须执行 deterministic fake-upstream smoke，并把真实 smoke 明确标为 optional，而不是静默跳过全部覆盖。

- [ ] **Step 7: 提交**

```bash
git add openai-compatible-proxy-spec.md architecture/overview.md docs/adr/0002-codex-reasoning-dialect-bridge.md tasks/185-codex-reasoning-dialect-bridge.md README.md CHANGELOG.md docs/local-verification.md docs/ci.md docs/release.md scripts/smoke-deepseek.sh Makefile
git commit -m "docs: document Codex reasoning replay"
```

---

### Task 11: 全量验证、安全回归与交付检查

**Files:**
- Verify all files changed by Tasks 1–10
- Modify only files required to fix discovered regressions

- [ ] **Step 1: 检查禁止项**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "rg -n 'reasoning_content|Authorization|ACTIVE_REASONING_KEY' internal cmd | head -200"
```

人工确认：`reasoning_content` 仅在 DeepSeek wire/envelope 边界和测试出现；没有日志输出 Authorization、raw reasoning 或 key value；API handler 没有 `provider == \"deepseek\"` 分支。

- [ ] **Step 2: 运行 race-free focused suite**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "go test ./internal/conversation ./internal/reasoningenvelope ./internal/provider/dialect ./internal/provider/deepseek ./internal/compat ./internal/router ./internal/config ./internal/responsestore ./internal/api ./cmd/gateway -count=1"
```

Expected: PASS。

- [ ] **Step 3: 运行标准完整验证**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/codex-reasoning-dialect -- bash -lc "make verify"
```

Expected: PASS，包含格式、静态检查、单元/集成测试和仓库既有验证入口。

- [ ] **Step 4: 运行服务级本地验证**

按 `docs/local-verification.md` 在 WSL 执行普通 Chat、普通 Responses、Codex DeepSeek tool replay、stream cancellation、retrieve/delete 和配置 check。确认旧 endpoint、鉴权失败、模型不存在、上游错误路径均不回归。

- [ ] **Step 5: 审查 diff 与提交边界**

Run:

```powershell
git status --short
git diff --check
git log --oneline --decorate -12
```

确认没有带入主工作区原有未提交文件、真实 API key、临时日志或生成物。必要修复必须先写回归测试，再单独提交：

```bash
git add <only-the-fix-files>
git commit -m "fix: close reasoning replay verification gaps"
```

- [ ] **Step 6: 请求代码审查**

使用 `superpowers:requesting-code-review` 对照设计 spec、该计划和实际 diff 审查。任何 P0/P1 或功能契约问题按 `superpowers:receiving-code-review` 处理，并重新运行 Step 2–4。

- [ ] **Step 7: 完成前最终验证**

使用 `superpowers:verification-before-completion` 再运行最新的 `make verify` 和 `make smoke-deepseek`，记录命令与结果。只有两者均通过，且可选真实凭据测试被清楚标注时，才能声明实现完成。

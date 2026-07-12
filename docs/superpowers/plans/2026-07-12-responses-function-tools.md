# Responses API Function Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the stateless `/v1/responses` compatibility layer with function definitions, tool choice, parallel function calls, function outputs, and typed function-call streaming events.

**Architecture:** Keep `provider.Provider` unchanged. Extend `internal/compat` to translate between Responses function Items and Chat Completions `tools`/`tool_calls`, then extend the Responses handler's stream state machine to aggregate indexed chat tool-call deltas into Responses Items and events.

**Tech Stack:** Go 1.22+, `encoding/json`, `net/http`, standard-library tests, Bash smoke tests, WSL `Ubuntu-24.04`.

## Global Constraints

- Use only the Go standard library and existing project dependencies.
- Support only JSON Schema function tools; reject built-in, custom, namespace, allowed-tools, tool-search, MCP, reasoning, and multimodal forms.
- Support zero, one, or multiple function calls and preserve stable `call_id` correlation.
- Omitted Responses `strict` becomes explicit `true`; explicit true/false is preserved.
- Do not execute functions in the gateway and do not add provider interface methods.
- Preserve existing text-only Responses, chat, embeddings, provider, fallback, audit, metrics, and cancellation behavior.
- Final verification runs in WSL `Ubuntu-24.04` with `make release-check`.

---

### Task 1: Chat tool-call wire types

**Files:**
- Modify: `internal/compat/types.go`
- Modify: `internal/compat/responses_test.go`

**Interfaces:**
- Produces: `ChatTool`, `ChatFunctionDefinition`, `ChatToolChoice`, `ChatToolCall`, `ChatFunctionCall`, and indexed `ChatToolCallDelta` JSON types.
- Extends: `ChatCompletionRequest` with `Tools`, `ToolChoice`, `ParallelToolCalls`; `ChatMessage` with tool-call wire data through its existing `Extra`; `ChatMessageDelta` with decoded tool-call deltas.

- [ ] **Step 1: Write failing JSON round-trip tests**

Assert chat request JSON contains nested `tools[].function`, tool choice, parallel flag, assistant `tool_calls`, tool message `tool_call_id`, and stream `delta.tool_calls` with indexes and partial arguments.

```go
func TestChatCompletionRequestRoundTripsTools(t *testing.T) {
    // Marshal a request with one function and assert the exact nested wire shape.
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/compat -run 'TestChat.*Tool' -count=1`
Expected: FAIL because the tool types/fields do not exist.

- [ ] **Step 3: Implement minimal typed wire support**

Add typed request fields to the known-field marshal/unmarshal path. Decode `tool_calls` from message/delta extras into typed slices while preserving unknown fields. Use pointers for optional `parallel_tool_calls` and `strict` so omission is distinguishable.

- [ ] **Step 4: Verify GREEN and commit**

Run: `go test ./internal/compat -count=1`
Expected: PASS.

```bash
git add internal/compat/types.go internal/compat/responses_test.go
git commit -m "feat: add chat function tool wire types"
```

### Task 2: Responses tools, choice, and input Item conversion

**Files:**
- Modify: `internal/compat/responses.go`
- Modify: `internal/compat/responses_test.go`

**Interfaces:**
- Extends: `ResponseRequest` with tools, raw tool choice, and optional parallel flag.
- Produces: strict `ChatRequest()` conversion for function definitions, four tool-choice forms, function-call Items, and function-call-output Items.

- [ ] **Step 1: Write failing valid conversion tests**

Cover strict omitted/true/false, auto/none/required/forced choice, parallel omitted/true/false, multiple tools, a single function call/output pair, and multiple calls/outputs. Assert exact chat tool definitions and assistant/tool message sequence.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/compat -run 'TestResponseRequest.*(Tool|Function)' -count=1`
Expected: FAIL because Responses currently rejects `tools` and function Items.

- [ ] **Step 3: Implement function definition and choice conversion**

Parse tools as raw objects, accept only `type:function`, require trimmed unique name and object parameters, default omitted strict to true, and emit nested Chat tools. Parse choice strings and forced function objects; confirm forced names exist.

- [ ] **Step 4: Implement stateless Item correlation**

Parse input as heterogeneous raw Items. Convert text messages as before. Track prior call IDs and completed outputs; reject duplicate calls, output-before-call, duplicate output, non-string output, invalid JSON arguments, and unsupported status. Merge consecutive function calls into one assistant message containing ordered tool calls; emit tool messages in input order.

- [ ] **Step 5: Write and pass invalid-request table tests**

Cover every 400 category from the design with exact parameter paths: tool type/name/parameters/strict, choice, parallel flag, call/output fields, ordering, duplicates, and arguments JSON.

- [ ] **Step 6: Verify and commit**

Run: `go test ./internal/compat -count=1`
Expected: PASS.

```bash
git add internal/compat/responses.go internal/compat/responses_test.go
git commit -m "feat: translate Responses function tool requests"
```

### Task 3: Non-streaming function-call output conversion

**Files:**
- Modify: `internal/compat/responses.go`
- Modify: `internal/compat/responses_test.go`
- Modify: `internal/api/responses_test.go`

**Interfaces:**
- Extends: `Response.Output` to a heterogeneous ordered Item list.
- Produces: `ResponseFunctionCall` and conversion from chat assistant content/tool calls.

- [ ] **Step 1: Write failing conversion tests**

Cover tool-only, text-only regression, text plus multiple tools, generated `fc_` IDs, preserved call IDs/order, usage, and malformed provider results (missing/duplicate ID, missing name/arguments, wrong type, invalid arguments JSON).

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/compat ./internal/api -run 'Test.*Response.*Function' -count=1`
Expected: FAIL because `NewResponseEnvelope` only accepts plain assistant text.

- [ ] **Step 3: Implement ordered heterogeneous output Items**

Represent output as `[]json.RawMessage` or a custom union marshal type. Add the message Item only for non-empty text; append one completed function Item per chat tool call. Keep exactly one choice and generate Items through an injected ID factory or deterministic helper usable in tests.

- [ ] **Step 4: Add API capture-provider tests**

Assert request forwarding and final HTTP JSON for one and multiple tool calls, while existing text response tests remain unchanged.

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/compat ./internal/api -count=1`
Expected: PASS.

```bash
git add internal/compat/responses.go internal/compat/responses_test.go internal/api/responses_test.go
git commit -m "feat: emit Responses function call Items"
```

### Task 4: Streaming function-call state machine

**Files:**
- Modify: `internal/api/responses.go`
- Modify: `internal/api/responses_test.go`
- Modify: `internal/provider/fake/fake.go`

**Interfaces:**
- Produces: per-chat-index function call state with stable output index, Item ID, call ID, name, and accumulated arguments.
- Produces: `response.function_call_arguments.delta/done` plus item added/done events.

- [ ] **Step 1: Write failing single-call stream test**

Use a test provider that emits indexed partial arguments. Assert event/data type matching, global sequence numbers, stable IDs/indexes, completed output, final JSON arguments, no `[DONE]`, and stream close.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/api -run 'TestResponsesStream.*Function' -count=1`
Expected: FAIL because tool-call deltas are currently rejected.

- [ ] **Step 3: Implement per-index aggregation and lifecycle**

On first index allocate output index/Item ID and emit item-added. Validate later ID/name consistency, append arguments, and emit argument deltas. Sort deltas within each chunk by chat index after emitting text. At EOF validate accumulated JSON, then emit arguments-done and item-done in output-index order before response-completed.

- [ ] **Step 4: Add multiple/interleaved and mixed text tests**

Assert independent accumulation for at least two indexes, deterministic order for out-of-order deltas, text-first behavior within a chunk, and final output order based on first appearance.

- [ ] **Step 5: Add stream error tests**

Cover conflicting ID/name, negative/ambiguous index, missing fields, invalid final arguments, typed error event, audit event body, provider-health effect, cancellation, and close.

- [ ] **Step 6: Verify and commit**

Run: `go test ./internal/api -count=1`
Expected: PASS.

```bash
git add internal/api/responses.go internal/api/responses_test.go internal/provider/fake/fake.go
git commit -m "feat: stream Responses function calls"
```

### Task 5: Offline two-turn tool smoke

**Files:**
- Create: `scripts/smoke-responses-tools.sh`
- Modify: `Makefile`
- Modify: `internal/provider/fake/fake.go`
- Create: `internal/provider/fake/fake_test.go`
- Modify: `docs/ci.md`
- Modify: `docs/local-verification.md`

**Interfaces:**
- Produces: `make smoke-responses-tools` on a unique default port, included in `release-check`.

- [ ] **Step 1: Write a failing fake-provider behavior test**

Define deterministic behavior: a request with tools and no tool result returns `get_weather` call; a request containing the correlated tool result returns final text. Assert normal fake behavior without tools is unchanged.

- [ ] **Step 2: Implement the minimal fake behavior and verify**

Run: `go test ./internal/provider/fake -count=1`
Expected: PASS.

- [ ] **Step 3: Create the two-turn smoke**

Start the gateway on `127.0.0.1:18085`, send the first Responses request, extract the deterministic call/item fields without `jq` by using stable expected values, send a second request containing the call and output, and assert final output text. Also test streaming arguments events.

- [ ] **Step 4: Wire Make/release/docs and run**

Run: `make smoke-responses-tools`
Expected: `responses-tools-smoke-ok`.

- [ ] **Step 5: Commit**

```bash
git add scripts/smoke-responses-tools.sh Makefile internal/provider/fake docs/ci.md docs/local-verification.md
git commit -m "test: add Responses function tools smoke"
```

### Task 6: Contract documentation and complete verification

**Files:**
- Create: `tasks/158-responses-function-tools.md`
- Modify: `README.md`
- Modify: `openai-compatible-proxy-spec.md`
- Modify: `architecture/overview.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Produces: public Chinese contract matching the implementation and a completed Task 158 record.

- [ ] **Step 1: Update public contract and architecture**

Document tools, strict default, four choices, parallel calls, stateless correlation, non-stream Items, streaming events, errors, supported provider path, and explicit non-goals.

- [ ] **Step 2: Add Task 158 and changelog**

Record acceptance checks and mark Done only after all verification below passes.

- [ ] **Step 3: Run focused and full WSL verification**

Run: `wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/compat ./internal/api ./internal/provider/fake -count=1 && make smoke-responses-tools && make release-check"`
Expected: all tests, race, vet, build, config checks, and smoke targets PASS.

- [ ] **Step 4: Inspect and commit**

Run: `git diff --check && git status --short`
Expected: only intended documentation or verification fixes remain.

```bash
git add tasks/158-responses-function-tools.md README.md openai-compatible-proxy-spec.md architecture/overview.md CHANGELOG.md
git commit -m "docs: document Responses function tools"
```

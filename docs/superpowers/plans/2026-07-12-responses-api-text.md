# Responses API Minimal Text Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a stateless, text-only `POST /v1/responses` endpoint that maps to the existing Chat Completions provider pipeline and emits OpenAI-compatible JSON and typed SSE responses.

**Architecture:** Define strict Responses request/response/event types and pure conversion helpers in `internal/compat`, then add a focused API handler that reuses the existing chat routing, fallback, health, timeout, audit, and metrics machinery with a caller-supplied path label. Keep every provider adapter and the `provider.Provider` interface unchanged.

**Tech Stack:** Go 1.22+, `net/http`, `encoding/json`, standard-library tests, Bash smoke tests, WSL `Ubuntu-24.04`.

## Global Constraints

- Use the Go standard library; add no runtime dependency.
- Accept only text input, message arrays, optional `instructions`, boolean `stream`, and omitted/false `store`.
- Reject every unsupported or unknown field with stable `400 invalid_request_error`.
- Do not add Responses methods or provider-specific logic to provider adapters.
- Preserve context cancellation and close upstream streams.
- Update the public contract, architecture, README, task, local verification, CI/release documentation, and changelog.
- Run final verification in WSL `Ubuntu-24.04`.

---

### Task 1: Strict Responses compatibility types and conversion

**Files:**
- Create: `internal/compat/responses.go`
- Create: `internal/compat/responses_test.go`

**Interfaces:**
- Produces: `ResponseRequest.UnmarshalJSON`, `ResponseRequest.Validate() *Error`, `ResponseRequest.ChatRequest() (ChatCompletionRequest, *Error)`.
- Produces: `NewResponseEnvelope(externalModel string, chat *ChatCompletionResponse, now time.Time, responseID, messageID string) (*Response, *Error)`.
- Produces: response, message, content-part, usage, and typed stream event structs used by Task 3.

- [ ] **Step 1: Write failing request parsing and conversion tests**

Cover string input, message input with string content, `input_text` parts, `instructions`, missing fields, empty text, invalid roles, `store:true`, unknown top-level fields, and representative unsupported Items. Assert exact `ChatMessage` order and exact error `Type`, `Param`, and message.

```go
func TestResponseRequestStringInputToChat(t *testing.T) {
    var req ResponseRequest
    err := json.Unmarshal([]byte(`{"model":"test-model","instructions":"be concise","input":"hello"}`), &req)
    // assert no error, Validate succeeds, and ChatRequest messages are developer then user.
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/compat -run 'TestResponse' -count=1`
Expected: FAIL because Responses types do not exist.

- [ ] **Step 3: Implement strict decoding, validation, and request conversion**

Use `json.RawMessage` to distinguish input string from array. Decode top-level fields from a raw map, delete exactly `model`, `input`, `instructions`, `stream`, and `store`, then reject the lexicographically first remaining key. Accept message content only as a non-empty string or a non-empty array of `{ "type": "input_text", "text": "..." }`. Marshal converted text into `ChatMessage.Content` with `json.Marshal`.

- [ ] **Step 4: Write failing response conversion tests**

Assert `object:"response"`, external model, one completed assistant message Item, `output_text`, empty annotations, and prompt/completion/total to input/output/total usage mapping. Reject zero/multiple choices, non-assistant role, null/non-string content, and chat tool-call extras.

- [ ] **Step 5: Implement response and stream event types**

Define stable JSON structs for `Response`, `ResponseOutputMessage`, `ResponseOutputText`, `ResponseUsage`, and the nine text lifecycle event payloads. Do not add an `output_text` JSON field to `Response`; SDKs derive it from `output`.

- [ ] **Step 6: Run tests and commit**

Run: `go test ./internal/compat -run 'TestResponse' -count=1`
Expected: PASS.

```bash
git add internal/compat/responses.go internal/compat/responses_test.go
git commit -m "feat: add Responses text compatibility types"
```

### Task 2: Route registration and path-aware chat provider helpers

**Files:**
- Modify: `internal/routes/routes.go`
- Modify: `internal/routes/routes_test.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/chat_completions.go`
- Modify: `internal/api/chat_completions_test.go`

**Interfaces:**
- Produces: `routes.ResponsesPath = "/v1/responses"` registered for POST.
- Produces: chat fallback/open helpers that accept `path string`, while existing chat behavior remains identical.

- [ ] **Step 1: Write failing route tests**

Assert normalization, POST allowance, GET rejection with `Allow: POST`, authentication, and handler-map completeness for `/v1/responses`.

- [ ] **Step 2: Run route tests and verify RED**

Run: `go test ./internal/routes ./internal/api -run 'ResponsesRoute|MethodNotAllowed' -count=1`
Expected: FAIL because `ResponsesPath` is absent.

- [ ] **Step 3: Register the route and handler placeholder**

Add `ResponsesPath` to route definitions and `s.handleResponses` to `routeHandlers`. Add the real method in Task 3 within the same working tree before package compilation is expected to pass.

- [ ] **Step 4: Parameterize shared provider attempt helpers by path**

Change `createChatCompletionWithFallback` and `openChatCompletionStreamWithFallback` to accept a `path string`; replace internal `routes.ChatCompletionsPath` metric observations with that argument. Existing chat callers pass `routes.ChatCompletionsPath`; Task 3 passes `routes.ResponsesPath`.

- [ ] **Step 5: Run existing chat and route tests after Task 3 compiles, then commit with Task 3**

Run: `go test ./internal/routes ./internal/api -count=1`
Expected: PASS with unchanged chat metrics and method behavior.

### Task 3: Non-streaming and streaming Responses API handler

**Files:**
- Create: `internal/api/responses.go`
- Create: `internal/api/responses_test.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/chat_completions.go`

**Interfaces:**
- Consumes: Task 1 Responses types/converters.
- Consumes: Task 2 path-aware provider helpers.
- Produces: `(*Server).handleResponses`, non-stream JSON responses, and typed SSE lifecycle events.

- [ ] **Step 1: Write failing non-streaming API tests**

Test valid string input, messages/instructions conversion through a capture provider, response IDs and model, usage metrics under `path="/v1/responses"`, auth failure, client allowlist, model-not-found, capability error, provider errors, fallback, and timeout.

- [ ] **Step 2: Implement the non-streaming handler**

Follow the chat handler lifecycle: require JSON content type, bounded strict decode, set log stream, validate, enforce client model visibility, resolve `chat`, audit original request, create a request-timeout context, call the path-aware helper, convert the result, audit the Responses envelope, and encode JSON.

- [ ] **Step 3: Run non-streaming tests**

Run: `go test ./internal/api -run 'TestResponses' -count=1`
Expected: all Responses tests written so far PASS.

- [ ] **Step 4: Write failing typed SSE tests**

Parse SSE blocks and assert exact event order, matching `event:` and payload `type`, stable response/message IDs and indexes, strictly increasing `sequence_number`, accumulated done text, no `[DONE]`, final usage, fallback on stream-open failure, post-start error event, timeout health failure, cancellation, and stream close.

- [ ] **Step 5: Implement typed SSE lifecycle writing**

Add `writeTypedSSE(w io.Writer, eventType string, value any) error`; it writes `event: ` plus the supplied event type, then `data: ` plus the JSON payload, with a blank line terminator. Emit created/in-progress/item/content events after the provider stream opens, convert text deltas while accumulating text and latest usage, emit done/completed events on EOF, emit an `error` event after start failures, and flush every event.

- [ ] **Step 6: Run API tests and commit Tasks 2-3**

Run: `go test ./internal/routes ./internal/api -count=1`
Expected: PASS.

```bash
git add internal/routes internal/api/server.go internal/api/chat_completions.go internal/api/chat_completions_test.go internal/api/responses.go internal/api/responses_test.go
git commit -m "feat: add minimal Responses API endpoint"
```

### Task 4: Audit and observability contract coverage

**Files:**
- Modify: `internal/api/responses_test.go`
- Modify: `internal/middleware/metrics_test.go` only if generic path normalization needs explicit coverage

**Interfaces:**
- Verifies: request/response/chunk/error audit events and all relevant metric labels use `/v1/responses`.

- [ ] **Step 1: Add failing audit and metrics assertions**

Assert request body, response body, stream chunk/event bodies, provider/upstream model, client, status, token/cost totals, fallback counter, circuit-open counter, HTTP count, and rate-limit rejection labels.

- [ ] **Step 2: Run focused tests and verify failures identify missing observations**

Run: `go test ./internal/api -run 'TestResponses.*(Audit|Metrics|Fallback|Circuit|RateLimit)' -count=1`
Expected: FAIL for any missing path-aware observation.

- [ ] **Step 3: Complete minimal observation wiring and run tests**

Use existing `auditBaseEvent`, `observeUsage`, `observeProviderFallback`, `observeProviderCircuitOpen`, and middleware metrics. Do not add new metric names or labels.

- [ ] **Step 4: Commit**

```bash
git add internal/api/responses.go internal/api/responses_test.go internal/middleware/metrics_test.go
git commit -m "test: cover Responses API observability"
```

### Task 5: Credential-free service smoke and release integration

**Files:**
- Create: `scripts/smoke-responses.sh`
- Modify: `Makefile`
- Modify: `docs/ci.md`
- Modify: `docs/local-verification.md`

**Interfaces:**
- Produces: `make smoke-responses`, included in `release-check`.

- [ ] **Step 1: Add the target and failing smoke script assertions**

Start the fake gateway on an isolated port with a trap cleanup. Use curl to assert a non-stream response contains `"object":"response"`, `"type":"output_text"`, and expected fake text. Assert stream output includes `event: response.created`, `event: response.output_text.delta`, and `event: response.completed`, and excludes `[DONE]`.

- [ ] **Step 2: Run smoke and verify RED before endpoint implementation is complete**

Run: `bash scripts/smoke-responses.sh`
Expected: non-zero until the endpoint is available.

- [ ] **Step 3: Complete script, Make target, release-check, and docs**

Add `smoke-responses` to `.PHONY`; add `smoke-responses: bash scripts/smoke-responses.sh`; include it in `release-check`. Document optional official SDK checks separately so standard validation remains offline.

- [ ] **Step 4: Run and commit**

Run: `make smoke-responses`
Expected: PASS with a final success message.

```bash
git add scripts/smoke-responses.sh Makefile docs/ci.md docs/local-verification.md
git commit -m "test: add Responses API smoke coverage"
```

### Task 6: Public contract, architecture, task, and release notes

**Files:**
- Create: `tasks/157-responses-api-text.md`
- Modify: `README.md`
- Modify: `openai-compatible-proxy-spec.md`
- Modify: `architecture/overview.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Produces: Chinese documentation that exactly describes the supported subset and stable rejection behavior.

- [ ] **Step 1: Update all contract documents**

Document `POST /v1/responses`, accepted fields and text input shapes, response envelope, nine-event stream lifecycle, no `[DONE]`, `store:false`, chat capability routing, provider translation, and the full rejected-capability categories.

- [ ] **Step 2: Add Task 157 acceptance record and changelog entry**

Task status remains actionable until implementation verification completes, then becomes `Done`. Add the feature under `CHANGELOG.md` Unreleased.

- [ ] **Step 3: Check documentation consistency and commit**

Run: `rg -n "/v1/responses|smoke-responses|previous_response_id|response.completed" README.md openai-compatible-proxy-spec.md architecture docs tasks CHANGELOG.md`
Expected: every public surface contains the intended contract with no contradictory support claim.

```bash
git add tasks/157-responses-api-text.md README.md openai-compatible-proxy-spec.md architecture/overview.md CHANGELOG.md
git commit -m "docs: document minimal Responses API support"
```

### Task 7: Full verification and completion

**Files:**
- Modify only files required by failures caused by this feature.

**Interfaces:**
- Verifies: the entire repository and release path.

- [ ] **Step 1: Run formatting and focused tests in WSL**

Run: `wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "gofmt -w internal/compat/responses.go internal/compat/responses_test.go internal/api/responses.go internal/api/responses_test.go && go test ./internal/compat ./internal/routes ./internal/api -count=1"`
Expected: PASS.

- [ ] **Step 2: Run full verification in WSL**

Run: `wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "make verify"`
Expected: PASS for line endings, tests, race, and vet.

- [ ] **Step 3: Run service and release verification in WSL**

Run: `wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "make smoke-responses && make release-check"`
Expected: PASS without real provider credentials or external network.

- [ ] **Step 4: Inspect diff and commit verification fixes**

Run: `git status --short && git diff --check`
Expected: no unintended files and no whitespace errors.

If verification requires a code or documentation fix, stage exactly the paths shown by `git status --short`, inspect the staged diff with `git diff --cached`, and commit them with `git commit -m "fix: complete Responses API verification"`. Skip the final commit if verification required no changes.

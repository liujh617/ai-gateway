# Image Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `POST /v1/images/generations` endpoint with JSON-only request/response, reusing existing model router, provider fallback, circuit breaker, and metrics infrastructure.

**Tech Stack:** Go 1.22 standard library; WSL `Ubuntu-24.04` for all verification.

## Global Constraints

- Prefer the Go standard library and add no dependency.
- JSON-only (no multipart form-data).
- Reuse existing middleware, fallback, circuit breaker, metrics, audit.
- Add `images` capability to model configuration.
- TDD: write failing tests first, then implement.
- Run final verification via `make verify` in WSL.

---

### Task 1: Add compat types with TDD

**Files:**
- Modify: `internal/compat/types.go`
- New test file or add to existing tests

- [ ] **Step 1: Write failing ImageGenerationRequest/Response tests**

```go
func TestImageGenerationRequestJSON(t *testing.T) {
    req := ImageGenerationRequest{Model: "dall-e-3", Prompt: json.RawMessage(`"a cat"`), N: intPtr(1), Size: "1024x1024"}
    payload, _ := json.Marshal(req)
    var parsed ImageGenerationRequest
    json.Unmarshal(payload, &parsed)
    // verify round-trip
}

func TestImageGenerationRequestValidate(t *testing.T) {
    // empty model, empty prompt → error
}
```

- [ ] **Step 2: Implement ImageGenerationRequest/Response types**

Add to `internal/compat/types.go`: `ImageGenerationRequest`, `ImageGenerationResponse` with known-fields + Extra pattern.

---

### Task 2: Extend Provider interface + implementations

**Files:**
- Modify: `internal/provider/provider.go`
- Modify: `internal/provider/fake/fake.go`
- Modify: `internal/provider/openai/openai.go`
- Modify: `internal/provider/azureopenai/azureopenai.go`

- [ ] **Step 3: Add CreateImage to Provider interface**
- [ ] **Step 4: Add fake CreateImage implementation**
- [ ] **Step 5: Add openai CreateImage implementation**
- [ ] **Step 6: Add azure openai CreateImage implementation**

---

### Task 3: Routes, API handler, and server registration

**Files:**
- Modify: `internal/routes/routes.go`, `internal/routes/routes_test.go`
- Create: `internal/api/images.go`
- Add tests to `internal/api/` test files
- Modify: `internal/api/server.go`

- [ ] **Step 7: Add CompletionsPath route constant and definition**
- [ ] **Step 8: Create handleImages with createImageWithFallback**
- [ ] **Step 9: Register route in server.go**
- [ ] **Step 10: Write API tests (success, validation, auth, capability, fallback, circuit-open)**

---

### Task 4: Config, spec, docs, and final verification

**Files:**
- Modify: `internal/config/config.go`, `internal/config/config_test.go`
- Modify: `openai-compatible-proxy-spec.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Create: `tasks/172-image-generation.md`

- [ ] **Step 11: Add "images" capability to config**
- [ ] **Step 12: Update spec, README, CHANGELOG**
- [ ] **Step 13: Run make verify**
- [ ] **Step 14: Commit**

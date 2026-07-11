# Agent Audit JSONL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an explicitly enabled local JSONL audit mode that records full agent request/response bodies for research.

**Architecture:** Add a small `internal/audit` package for JSONL event writing, load audit config in `internal/config`, wire an optional recorder into `cmd/gateway` and `internal/api`, then emit request/response/stream events from chat and embeddings handlers. The provider layer stays unchanged.

**Tech Stack:** Go standard library, `encoding/json`, `os`, `sync`, `net/http`, existing `net/http` gateway, existing WSL verification flow.

---

## File Structure

- Create `internal/audit/audit.go`: event types, JSONL recorder, trace id helper, no dependency on API/provider packages.
- Create `internal/audit/audit_test.go`: unit tests for JSONL writing, disabled recorder behavior, trace id fallback.
- Modify `internal/config/config.go`: add `AuditConfig`, defaults, env overrides, validation, check report fields.
- Modify `internal/config/config_test.go`: audit config tests and check report tests.
- Modify `schema/config.schema.json`: schema for `audit.enabled` and `audit.path`.
- Modify `internal/config/examples_test.go`: ensure schema/existing example checks still cover audit schema.
- Modify `cmd/gateway/main.go`: build audit recorder, pass to API options, close on shutdown, include startup log fields.
- Modify `internal/api/server.go`: add optional `AuditRecorder` to `Options` and `Server`.
- Modify `internal/api/chat_completions.go`: emit chat request, response, stream_chunk, stream_done, error events.
- Modify `internal/api/embeddings.go`: emit embeddings request, response, error events.
- Modify `internal/api/chat_completions_test.go`: API-level audit tests.
- Modify `openai-compatible-proxy-spec.md`, `architecture/overview.md`, `README.md`, `docs/local-verification.md`: document local research audit mode.
- Add task records `tasks/141-agent-audit-jsonl-recorder.md` through `tasks/148-agent-audit-docs.md` as each task is implemented.

---

### Task 1: Add Audit JSONL Recorder

**Files:**
- Create: `internal/audit/audit.go`
- Create: `internal/audit/audit_test.go`
- Create: `tasks/141-agent-audit-jsonl-recorder.md`

- [ ] **Step 1: Write failing recorder tests**

Create `internal/audit/audit_test.go`:

```go
package audit_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/requestctx"
)

func TestJSONLRecorderWritesEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit", "agent.jsonl")
	rec, err := audit.NewJSONLRecorder(path)
	if err != nil {
		t.Fatalf("NewJSONLRecorder: %v", err)
	}
	defer rec.Close()

	body := json.RawMessage(`{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`)
	rec.Record(context.Background(), audit.Event{
		Timestamp:     time.Date(2026, 7, 11, 1, 2, 3, 4, time.UTC),
		Event:         audit.EventRequest,
		RequestID:     "req_1",
		TraceID:       "trace_1",
		Path:          "/v1/chat/completions",
		Client:        "alpha",
		ExternalModel: "test-model",
		Provider:      "fake-provider",
		UpstreamModel: "upstream-test-model",
		Body:          body,
	})

	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("missing audit line")
	}
	var got map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatalf("decode audit line: %v", err)
	}
	if got["event"] != "request" || got["request_id"] != "req_1" || got["trace_id"] != "trace_1" {
		t.Fatalf("audit event = %#v", got)
	}
	bodyMap, ok := got["body"].(map[string]any)
	if !ok || bodyMap["model"] != "test-model" {
		t.Fatalf("body = %#v", got["body"])
	}
	if scanner.Scan() {
		t.Fatalf("unexpected second audit line: %s", scanner.Text())
	}
}

func TestNoopRecorderDoesNotCreateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.jsonl")
	var rec audit.NoopRecorder
	rec.Record(context.Background(), audit.Event{Event: audit.EventRequest, Body: json.RawMessage(`{"ok":true}`)})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stat = %v, want not exist", err)
	}
}

func TestTraceIDFromRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req = req.WithContext(requestctx.WithRequestID(req.Context(), "req_fallback"))
	if got := audit.TraceIDFromRequest(req); got != "req_fallback" {
		t.Fatalf("trace id fallback = %q", got)
	}
	req.Header.Set(audit.TraceIDHeader, " agent-session-1 ")
	if got := audit.TraceIDFromRequest(req); got != "agent-session-1" {
		t.Fatalf("trace id header = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/audit -count=1"
```

Expected: FAIL because `internal/audit` does not exist.

- [ ] **Step 3: Implement recorder**

Create `internal/audit/audit.go`:

```go
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"open-ai-gateway/internal/requestctx"
)

const TraceIDHeader = "X-Agent-Trace-Id"

const (
	EventRequest     = "request"
	EventResponse    = "response"
	EventStreamChunk = "stream_chunk"
	EventStreamDone  = "stream_done"
	EventError       = "error"
)

type Event struct {
	Timestamp     time.Time       `json:"timestamp"`
	Event         string          `json:"event"`
	RequestID     string          `json:"request_id,omitempty"`
	TraceID       string          `json:"trace_id,omitempty"`
	Path          string          `json:"path,omitempty"`
	Client        string          `json:"client,omitempty"`
	ExternalModel string          `json:"external_model,omitempty"`
	Provider      string          `json:"provider,omitempty"`
	UpstreamModel string          `json:"upstream_model,omitempty"`
	Status        int             `json:"status,omitempty"`
	DurationMS    int64           `json:"duration_ms,omitempty"`
	Body          json.RawMessage `json:"body,omitempty"`
	Error         string          `json:"error,omitempty"`
}

type Recorder interface {
	Record(ctx context.Context, event Event)
	Close() error
}

type NoopRecorder struct{}

func (NoopRecorder) Record(context.Context, Event) {}
func (NoopRecorder) Close() error                  { return nil }

type JSONLRecorder struct {
	mu     sync.Mutex
	file   *os.File
	logger *slog.Logger
	now    func() time.Time
}

func NewJSONLRecorder(path string) (*JSONLRecorder, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &JSONLRecorder{
		file:   file,
		logger: slog.Default(),
		now:    time.Now,
	}, nil
}

func (r *JSONLRecorder) Record(ctx context.Context, event Event) {
	if r == nil || r.file == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = r.now().UTC()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		r.logger.Debug("failed to marshal audit event", "error", err)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.file.Write(append(payload, '\n')); err != nil {
		r.logger.Debug("failed to write audit event", "error", err)
	}
}

func (r *JSONLRecorder) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	err := r.file.Close()
	r.file = nil
	return err
}

func TraceIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if traceID := strings.TrimSpace(r.Header.Get(TraceIDHeader)); traceID != "" {
		return traceID
	}
	return requestctx.RequestID(r.Context())
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/audit -count=1"
```

Expected: PASS.

- [ ] **Step 5: Commit**

Create `tasks/141-agent-audit-jsonl-recorder.md` describing the recorder. Then run:

```powershell
git add internal/audit/audit.go internal/audit/audit_test.go tasks/141-agent-audit-jsonl-recorder.md
git commit -m "feat: add audit jsonl recorder"
```

---

### Task 2: Add Audit Configuration and Check Report

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `schema/config.schema.json`
- Create: `tasks/142-agent-audit-config.md`

- [ ] **Step 1: Write failing config tests**

Add tests to `internal/config/config_test.go` near existing env/config tests:

```go
func TestLoadConfigDefaultsAuditDisabled(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load default: %v", err)
	}
	if cfg.Audit.Enabled {
		t.Fatal("audit enabled by default")
	}
	if cfg.Audit.Path != "audit/agent-trace.jsonl" {
		t.Fatalf("audit path = %q", cfg.Audit.Path)
	}
}

func TestLoadConfigAcceptsAuditConfig(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"audit": {
			"enabled": true,
			"path": "tmp/audit.jsonl"
		},
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "fake"
			}
		}
	}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Audit.Enabled || cfg.Audit.Path != "tmp/audit.jsonl" {
		t.Fatalf("audit config = %#v", cfg.Audit)
	}
}

func TestEnvironmentAuditOverrides(t *testing.T) {
	t.Setenv("GATEWAY_AUDIT_ENABLED", "yes")
	t.Setenv("GATEWAY_AUDIT_PATH", "tmp/env-audit.jsonl")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load default: %v", err)
	}
	if !cfg.Audit.Enabled || cfg.Audit.Path != "tmp/env-audit.jsonl" {
		t.Fatalf("audit config = %#v", cfg.Audit)
	}
}

func TestEnvironmentAuditEnabledRejectsInvalidValue(t *testing.T) {
	t.Setenv("GATEWAY_AUDIT_ENABLED", "maybe")

	_, err := config.Load("")
	if err == nil {
		t.Fatal("expected audit validation error")
	}
	if !strings.Contains(err.Error(), "GATEWAY_AUDIT_ENABLED") {
		t.Fatalf("error = %v", err)
	}
}
```

Extend `TestCheckReportDoesNotExposeAPIKey` expectations with:

```go
`"audit_enabled":false`,
`"audit_path":"audit/agent-trace.jsonl"`,
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/config -run 'TestLoadConfigDefaultsAuditDisabled|TestLoadConfigAcceptsAuditConfig|TestEnvironmentAuditOverrides|TestEnvironmentAuditEnabledRejectsInvalidValue' -count=1"
```

Expected: FAIL because `Config.Audit` does not exist.

- [ ] **Step 3: Implement config**

Modify `internal/config/config.go`:

```go
type Config struct {
	...
	Audit                    AuditConfig               `json:"audit"`
	...
}

type AuditConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}
```

Add check report fields:

```go
AuditEnabled bool   `json:"audit_enabled"`
AuditPath    string `json:"audit_path"`
```

Set report fields in `CheckReport()`:

```go
AuditEnabled: c.Audit.Enabled,
AuditPath:    c.Audit.Path,
```

In `applyDefaults()` add:

```go
if c.Audit.Path == "" {
	c.Audit.Path = "audit/agent-trace.jsonl"
}
if env := os.Getenv("GATEWAY_AUDIT_ENABLED"); env != "" {
	enabled, err := parseBoolEnv(env)
	if err != nil {
		c.Audit.Enabled = false
		c.Audit.Path = strings.TrimSpace(c.Audit.Path)
		c.auditEnvError = err // do not add this hidden field; instead use package-level helper in Validate as below
	}
	c.Audit.Enabled = enabled
}
if env := os.Getenv("GATEWAY_AUDIT_PATH"); env != "" {
	c.Audit.Path = env
}
```

Do not add a hidden field. Instead, implement env parsing before validation directly in `applyDefaults` with a private helper that returns `(bool, bool)` and stores invalid state by setting an invalid sentinel value:

```go
const invalidAuditEnabledValue = "__invalid_audit_enabled__"

func parseAuditEnabledEnv(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}
```

Use it in `applyDefaults()`:

```go
if env := os.Getenv("GATEWAY_AUDIT_ENABLED"); env != "" {
	if enabled, ok := parseAuditEnabledEnv(env); ok {
		c.Audit.Enabled = enabled
	} else {
		c.Audit.Path = invalidAuditEnabledValue
	}
}
```

Add validation:

```go
if c.Audit.Path == invalidAuditEnabledValue {
	return fmt.Errorf("GATEWAY_AUDIT_ENABLED must be one of 1, true, yes, on, 0, false, no, or off")
}
if strings.TrimSpace(c.Audit.Path) == "" {
	return fmt.Errorf("audit.path must be non-empty")
}
if c.Audit.Path != strings.TrimSpace(c.Audit.Path) {
	return fmt.Errorf("audit.path must not contain leading or trailing whitespace")
}
```

Update schema with:

```json
"audit": {
  "$ref": "#/$defs/audit"
}
```

and:

```json
"audit": {
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "enabled": {
      "type": "boolean"
    },
    "path": {
      "type": "string",
      "minLength": 1
    }
  }
}
```

- [ ] **Step 4: Run tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/config"
```

Expected: PASS.

- [ ] **Step 5: Commit**

Create `tasks/142-agent-audit-config.md`, then:

```powershell
git add internal/config/config.go internal/config/config_test.go schema/config.schema.json tasks/142-agent-audit-config.md
git commit -m "feat: add audit config"
```

---

### Task 3: Wire Audit Recorder in Command and API Options

**Files:**
- Modify: `cmd/gateway/main.go`
- Modify: `internal/api/server.go`
- Create: `tasks/143-agent-audit-wiring.md`

- [ ] **Step 1: Write failing command/config-check test if command tests exist**

If `cmd/gateway` has no tests, skip adding command tests for this task and rely on config/API tests plus `make check-config`. Do not introduce a large command test harness just for wiring.

- [ ] **Step 2: Implement API option**

Modify `internal/api/server.go`:

```go
import "open-ai-gateway/internal/audit"

type Server struct {
	...
	audit audit.Recorder
}

type Options struct {
	...
	Audit audit.Recorder
}
```

In `NewServer`:

```go
if opts.Audit == nil {
	opts.Audit = audit.NoopRecorder{}
}
...
audit: opts.Audit,
```

- [ ] **Step 3: Implement command wiring**

Modify `cmd/gateway/main.go` imports to include:

```go
"open-ai-gateway/internal/audit"
```

Before `api.NewServer`, add:

```go
auditRecorder, err := buildAuditRecorder(cfg)
if err != nil {
	logger.Error("failed to configure audit", "error", err)
	os.Exit(1)
}
defer auditRecorder.Close()
```

Pass it into options:

```go
Audit: auditRecorder,
```

Add startup log fields:

```go
"audit_enabled", cfg.Audit.Enabled,
"audit_path", cfg.Audit.Path,
```

Add helper:

```go
func buildAuditRecorder(cfg *config.Config) (audit.Recorder, error) {
	if !cfg.Audit.Enabled {
		return audit.NoopRecorder{}, nil
	}
	return audit.NewJSONLRecorder(cfg.Audit.Path)
}
```

- [ ] **Step 4: Run verification**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./cmd/gateway ./internal/api"
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "make check-config"
```

Expected: PASS, and config-check JSON includes audit fields from Task 2.

- [ ] **Step 5: Commit**

Create `tasks/143-agent-audit-wiring.md`, then:

```powershell
git add cmd/gateway/main.go internal/api/server.go tasks/143-agent-audit-wiring.md
git commit -m "feat: wire audit recorder"
```

---

### Task 4: Audit Non-Streaming Chat Completions

**Files:**
- Modify: `internal/api/chat_completions.go`
- Modify: `internal/api/server.go` if helper methods are shared there
- Modify: `internal/api/chat_completions_test.go`
- Create: `tasks/144-agent-audit-chat.md`

- [ ] **Step 1: Write failing API test**

Add helper test recorder near existing test helpers:

```go
type memoryAuditRecorder struct {
	mu     sync.Mutex
	events []audit.Event
}

func (r *memoryAuditRecorder) Record(ctx context.Context, event audit.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *memoryAuditRecorder) Close() error { return nil }

func (r *memoryAuditRecorder) Events() []audit.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]audit.Event(nil), r.events...)
}
```

Add test:

```go
func TestAuditChatCompletionsNonStream(t *testing.T) {
	rec := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: rec})
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(audit.TraceIDHeader, "trace-agent-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	events := rec.Events()
	if len(events) != 2 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Event != audit.EventRequest || events[0].TraceID != "trace-agent-1" {
		t.Fatalf("request audit = %#v", events[0])
	}
	if !strings.Contains(string(events[0].Body), `"messages"`) {
		t.Fatalf("request body = %s", events[0].Body)
	}
	if events[1].Event != audit.EventResponse || events[1].Status != http.StatusOK {
		t.Fatalf("response audit = %#v", events[1])
	}
	if !strings.Contains(string(events[1].Body), `"choices"`) {
		t.Fatalf("response body = %s", events[1].Body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/api -run TestAuditChatCompletionsNonStream -count=1"
```

Expected: FAIL because no audit events are recorded.

- [ ] **Step 3: Implement chat audit helpers**

In `internal/api/server.go`, add helper methods:

```go
func (s *Server) auditBaseEvent(r *http.Request, event string, path string, externalModel string) audit.Event {
	return audit.Event{
		Event:         event,
		RequestID:     requestctx.RequestID(r.Context()),
		TraceID:       audit.TraceIDFromRequest(r),
		Path:          path,
		Client:        clientFromContext(r.Context()),
		ExternalModel: externalModel,
	}
}

func rawBody(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return payload
}
```

Add imports:

```go
"open-ai-gateway/internal/audit"
"open-ai-gateway/internal/requestctx"
```

In `handleChatCompletions`, after route resolve and `externalModel := req.Model`, record request:

```go
requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.ChatCompletionsPath, externalModel)
requestEvent.Body = rawBody(req)
s.audit.Record(r.Context(), requestEvent)
```

After success response:

```go
responseEvent := s.auditBaseEvent(r, audit.EventResponse, routes.ChatCompletionsPath, externalModel)
responseEvent.Provider = middleware.ProviderFromContext(r.Context()) // if no helper exists, use route.ProviderName for first implementation
responseEvent.Status = http.StatusOK
responseEvent.Body = rawBody(resp)
s.audit.Record(r.Context(), responseEvent)
```

If middleware has no provider accessor, use the selected provider returned from fallback. Adjust `createChatCompletionWithFallback` to return provider name:

```go
func (s *Server) createChatCompletionWithFallback(...) (*compat.ChatCompletionResponse, string, string, error)
```

Return `attempt.ProviderName` and `attempt.UpstreamModel` on success. Update existing call sites and tests.

- [ ] **Step 4: Run API tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/api"
```

Expected: PASS.

- [ ] **Step 5: Commit**

Create `tasks/144-agent-audit-chat.md`, then:

```powershell
git add internal/api/server.go internal/api/chat_completions.go internal/api/chat_completions_test.go tasks/144-agent-audit-chat.md
git commit -m "feat: audit chat completions"
```

---

### Task 5: Audit Embeddings

**Files:**
- Modify: `internal/api/embeddings.go`
- Modify: `internal/api/chat_completions_test.go`
- Create: `tasks/145-agent-audit-embeddings.md`

- [ ] **Step 1: Write failing API test**

Add:

```go
func TestAuditEmbeddings(t *testing.T) {
	rec := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: rec})
	body := `{"model":"test-model","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	events := rec.Events()
	if len(events) != 2 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Event != audit.EventRequest || !strings.Contains(string(events[0].Body), `"input"`) {
		t.Fatalf("request audit = %#v", events[0])
	}
	if events[1].Event != audit.EventResponse || !strings.Contains(string(events[1].Body), `"embedding"`) {
		t.Fatalf("response audit = %#v", events[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/api -run TestAuditEmbeddings -count=1"
```

Expected: FAIL because embeddings audit is not implemented.

- [ ] **Step 3: Implement embeddings audit**

Mirror chat non-streaming logic in `internal/api/embeddings.go`.

Adjust `createEmbeddingWithFallback` to return provider name and upstream model:

```go
func (s *Server) createEmbeddingWithFallback(...) (*compat.EmbeddingResponse, string, string, error)
```

Record:

- `request` after route resolve
- `response` after successful response
- provider/upstream model on response event

- [ ] **Step 4: Run API tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/api"
```

Expected: PASS.

- [ ] **Step 5: Commit**

Create `tasks/145-agent-audit-embeddings.md`, then:

```powershell
git add internal/api/embeddings.go internal/api/chat_completions_test.go tasks/145-agent-audit-embeddings.md
git commit -m "feat: audit embeddings"
```

---

### Task 6: Audit Streaming Chat Completions

**Files:**
- Modify: `internal/api/chat_completions.go`
- Modify: `internal/api/chat_completions_test.go`
- Create: `tasks/146-agent-audit-streaming-chat.md`

- [ ] **Step 1: Write failing stream audit test**

Add:

```go
func TestAuditChatCompletionsStream(t *testing.T) {
	rec := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: rec})
	body := `{"model":"test-model","stream":true,"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	events := rec.Events()
	var requestSeen, chunkSeen, doneSeen bool
	for _, event := range events {
		switch event.Event {
		case audit.EventRequest:
			requestSeen = true
		case audit.EventStreamChunk:
			chunkSeen = true
			if !strings.Contains(string(event.Body), `"choices"`) {
				t.Fatalf("chunk body = %s", event.Body)
			}
		case audit.EventStreamDone:
			doneSeen = true
		}
	}
	if !requestSeen || !chunkSeen || !doneSeen {
		t.Fatalf("events = %#v", events)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/api -run TestAuditChatCompletionsStream -count=1"
```

Expected: FAIL because stream events are not recorded.

- [ ] **Step 3: Implement stream audit**

In `streamChatCompletion`, after each chunk model rewrite and before/after `writeSSE`, record:

```go
chunkEvent := s.auditBaseEvent(r, audit.EventStreamChunk, routes.ChatCompletionsPath, externalModel)
chunkEvent.Provider = providerName
chunkEvent.Status = http.StatusOK
chunkEvent.Body = rawBody(chunk)
s.audit.Record(r.Context(), chunkEvent)
```

When EOF sends `[DONE]`, record:

```go
doneEvent := s.auditBaseEvent(r, audit.EventStreamDone, routes.ChatCompletionsPath, externalModel)
doneEvent.Provider = providerName
doneEvent.Status = http.StatusOK
s.audit.Record(r.Context(), doneEvent)
```

For cancellation/deadline/non-EOF errors after stream starts, record `EventError` with `Error` set to `"context_canceled"`, `"context_deadline_exceeded"`, or `"stream_error"`.

- [ ] **Step 4: Run API tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/api"
```

Expected: PASS.

- [ ] **Step 5: Commit**

Create `tasks/146-agent-audit-streaming-chat.md`, then:

```powershell
git add internal/api/chat_completions.go internal/api/chat_completions_test.go tasks/146-agent-audit-streaming-chat.md
git commit -m "feat: audit streaming chat completions"
```

---

### Task 7: Error Events and Write Failure Behavior

**Files:**
- Modify: `internal/api/chat_completions.go`
- Modify: `internal/api/embeddings.go`
- Modify: `internal/api/chat_completions_test.go`
- Modify: `internal/audit/audit_test.go`
- Create: `tasks/147-agent-audit-errors.md`

- [ ] **Step 1: Write failing tests**

Add API test:

```go
func TestAuditChatCompletionsValidationError(t *testing.T) {
	rec := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: rec})
	body := `{"model":"test-model"}`
	rr := doJSONWithRecorder(handler, body, true, rec) // if this helper does not exist, use explicit httptest request

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
	events := rec.Events()
	if len(events) != 1 || events[0].Event != audit.EventError || events[0].Status != http.StatusBadRequest {
		t.Fatalf("events = %#v", events)
	}
	if !strings.Contains(string(events[0].Body), `"error"`) {
		t.Fatalf("error body = %s", events[0].Body)
	}
}
```

If `doJSONWithRecorder` is awkward, write explicit `httptest.NewRequest` in the test.

Add audit package test with a recorder whose file is closed before `Record`; assert it does not panic.

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/audit ./internal/api -run 'TestAuditChatCompletionsValidationError|TestJSONLRecorderWriteFailureDoesNotPanic' -count=1"
```

Expected: FAIL until error audit is implemented.

- [ ] **Step 3: Implement error helper**

Add server helper:

```go
func (s *Server) writeAuditedError(w http.ResponseWriter, r *http.Request, path string, externalModel string, err *compat.Error) {
	s.writeError(w, r, err)
	event := s.auditBaseEvent(r, audit.EventError, path, externalModel)
	event.Status = err.Status
	event.Body = rawBody(compat.ErrorResponseFor(err))
	s.audit.Record(r.Context(), event)
}
```

Use this helper in chat and embeddings only after request body has been decoded enough to know path/model. Do not audit unauthenticated requests, content-type failures, invalid JSON decode failures, unknown routes, or method-not-allowed in first version.

- [ ] **Step 4: Run tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/audit ./internal/api"
```

Expected: PASS.

- [ ] **Step 5: Commit**

Create `tasks/147-agent-audit-errors.md`, then:

```powershell
git add internal/audit/audit_test.go internal/api/chat_completions.go internal/api/embeddings.go internal/api/chat_completions_test.go tasks/147-agent-audit-errors.md
git commit -m "feat: audit model request errors"
```

---

### Task 8: Documentation, Examples, and Final Verification

**Files:**
- Modify: `README.md`
- Modify: `openai-compatible-proxy-spec.md`
- Modify: `architecture/overview.md`
- Modify: `docs/local-verification.md`
- Modify: `config.example.json` if useful
- Modify: `schema/config.schema.json` if not already done in Task 2
- Create: `tasks/148-agent-audit-docs.md`

- [ ] **Step 1: Update docs**

Document:

- Audit is local research mode.
- It records full request/response bodies.
- Default disabled.
- Config and env variables.
- `X-Agent-Trace-Id`.
- JSONL event examples.
- Warning that prompts, completions, tool schemas, embedding inputs, and embedding vectors may be written to disk.

- [ ] **Step 2: Add local verification snippet**

Add to `docs/local-verification.md`:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "GATEWAY_AUDIT_ENABLED=1 GATEWAY_AUDIT_PATH=tmp/agent-audit.jsonl make smoke && tail -n 5 tmp/agent-audit.jsonl"
```

- [ ] **Step 3: Run final verification**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/audit ./internal/config ./internal/api"
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "make verify"
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "make release-check"
```

Expected: all PASS.

- [ ] **Step 4: Commit**

Create `tasks/148-agent-audit-docs.md`, then:

```powershell
git add README.md openai-compatible-proxy-spec.md architecture/overview.md docs/local-verification.md config.example.json schema/config.schema.json tasks/148-agent-audit-docs.md
git commit -m "docs: document agent audit jsonl mode"
```

---

## Self-Review

- Spec coverage:
  - JSONL recorder: Task 1.
  - Config/env/check-report/schema: Task 2.
  - Command/API wiring: Task 3.
  - Chat non-streaming audit: Task 4.
  - Embeddings audit: Task 5.
  - Streaming audit: Task 6.
  - Error/write-failure behavior: Task 7.
  - Public docs/final verification: Task 8.
- Placeholder scan:
  - Implementation tasks contain concrete file paths, code sketches, verification commands, and commit messages.
  - Deferred items remain in the design spec, not in implementation tasks.
- Type consistency:
  - Event names use `audit.EventRequest`, `audit.EventResponse`, `audit.EventStreamChunk`, `audit.EventStreamDone`, `audit.EventError`.
  - Trace header uses `audit.TraceIDHeader`.
  - API options use `api.Options{Audit: rec}`.

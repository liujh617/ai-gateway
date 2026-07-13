# Retrieve Stored Response Endpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add authenticated `GET` and `HEAD /v1/responses/{response_id}` endpoints that return the complete locally stored Response without contacting a provider.

**Architecture:** Extend each in-memory response-store record with an immutable JSON snapshot of the completed `compat.Response`, while retaining the transcript used by `previous_response_id`. Add a client-only store lookup and a canonical dynamic route; the API handler returns the saved JSON and maps every invisible or unavailable record to the same 404 response.

**Tech Stack:** Go 1.22 standard library (`net/http`, `encoding/json`), existing `compat`, `responsestore`, `routes`, middleware, and API packages; WSL `Ubuntu-24.04` for verification.

## Global Constraints

- Prefer the Go standard library and add no dependency.
- Keep handler, compat, router, provider, and store boundaries intact.
- Preserve existing `previous_response_id` behavior and store TTL/LRU/client/model semantics.
- Never call a provider or inspect provider health when retrieving a stored Response.
- Unknown, expired, evicted, disabled, unstored, and cross-client IDs return the same `404 invalid_request_error` with `param: response_id`.
- Normalize concrete IDs to `/v1/responses/{response_id}` in logs and metrics.
- Store only successfully completed Responses; never store failed or cancelled streams.
- Update the public compatibility contract, architecture, README, changelog, and Task 164 record.
- Run final verification in WSL `Ubuntu-24.04` from `/mnt/e/code/ai-gateway`.

---

### Task 1: Store the complete Response snapshot

**Files:**
- Modify: `internal/responsestore/store.go`
- Test: `internal/responsestore/store_test.go`

**Interfaces:**
- Consumes: existing `Record`, `Store.Put`, `Store.Get`, TTL/LRU limits, and miss counters.
- Produces: `Record.Response json.RawMessage` and `func (s *Store) GetByID(id, client string) (Record, MissReason, bool)`.

- [ ] **Step 1: Write failing response-copy and client-only lookup tests**

Add focused tests to `internal/responsestore/store_test.go` using the package's existing enabled-store and fake-clock helpers:

```go
func TestStoreGetByIDReturnsClonedResponse(t *testing.T) {
	store := New(Config{TTL: time.Minute, MaxEntries: 2, MaxContextBytes: 1024, MaxTotalBytes: 2048}, nil)
	const want = `{"id":"resp_1","object":"response","store":true}`
	record := Record{ID: "resp_1", Client: "alpha", Model: "test-model", Response: json.RawMessage(want)}
	if err := store.Put(record); err != nil {
		t.Fatalf("Put: %v", err)
	}
	record.Response[0] = 'x'

	got, reason, ok := store.GetByID("resp_1", "alpha")
	if !ok || reason != "" {
		t.Fatalf("GetByID ok=%v reason=%q", ok, reason)
	}
	if string(got.Response) != want {
		t.Fatalf("response = %s", got.Response)
	}
	got.Response[0] = 'x'
	again, _, _ := store.GetByID("resp_1", "alpha")
	if string(again.Response) != want {
		t.Fatalf("stored response mutated: %s", again.Response)
	}
}

func TestStoreGetByIDEnforcesClient(t *testing.T) {
	store := New(Config{TTL: time.Minute, MaxEntries: 2, MaxContextBytes: 1024, MaxTotalBytes: 2048}, nil)
	if err := store.Put(Record{ID: "resp_1", Client: "alpha", Model: "test-model", Response: json.RawMessage(`{"id":"resp_1"}`)}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, reason, ok := store.GetByID("resp_1", "beta"); ok || reason != MissClient {
		t.Fatalf("GetByID ok=%v reason=%q", ok, reason)
	}
}
```

Extend the existing size-limit test with a record whose transcript fits but whose transcript plus `Response` exceeds `MaxContextBytes`; expect `ErrContextTooLarge`.

- [ ] **Step 2: Run the store tests and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/responsestore -run 'GetByID|ContextTooLarge' -count=1"
```

Expected: build failure because `Record.Response` and `GetByID` do not exist.

- [ ] **Step 3: Implement response cloning, size accounting, and shared lookup**

Add the field:

```go
type Record struct {
	ID         string
	Client     string
	Model      string
	Transcript []compat.ChatMessage
	Response   json.RawMessage
}
```

Replace `encodedTranscriptSize` with record-wide accounting:

```go
func encodedRecordSize(record Record) (int64, error) {
	transcript, err := json.Marshal(record.Transcript)
	if err != nil {
		return 0, err
	}
	return int64(len(transcript) + len(record.Response)), nil
}
```

Call `encodedRecordSize(cloned)` from `Put`. Extend `cloneRecord` with:

```go
cloned.Response = cloneRaw(record.Response)
```

Refactor lookup so public methods do not double-count misses:

```go
func (s *Store) Get(id, client, model string) (Record, MissReason, bool) {
	return s.get(id, client, &model)
}

func (s *Store) GetByID(id, client string) (Record, MissReason, bool) {
	return s.get(id, client, nil)
}

func (s *Store) get(id, client string, model *string) (Record, MissReason, bool) {
	if s == nil || !s.config.enabled() {
		return Record{}, MissNotFound, false
	}
	s.mu.Lock()
	e, ok := s.entries[id]
	if !ok {
		s.misses[MissNotFound]++
		s.mu.Unlock()
		return Record{}, MissNotFound, false
	}
	if !s.clock.Now().Before(e.expiresAt) {
		s.removeLocked(e, EvictionExpired)
		s.misses[MissExpired]++
		s.mu.Unlock()
		return Record{}, MissExpired, false
	}
	if e.record.Client != client {
		s.misses[MissClient]++
		s.mu.Unlock()
		return Record{}, MissClient, false
	}
	if model != nil && e.record.Model != *model {
		s.misses[MissModel]++
		s.mu.Unlock()
		return Record{}, MissModel, false
	}
	s.lru.MoveToFront(e.lru)
	record := e.record
	s.mu.Unlock()
	return cloneRecord(record), "", true
}
```

- [ ] **Step 4: Run all store tests and verify GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/responsestore -count=1"
```

Expected: PASS, including existing continuation, eviction, stats, and concurrency tests.

- [ ] **Step 5: Commit the store capability**

```bash
git add internal/responsestore/store.go internal/responsestore/store_test.go
git commit -m "feat: store complete response snapshots"
```

### Task 2: Add the retrieve route and non-streaming behavior

**Files:**
- Modify: `internal/routes/routes.go`
- Test: `internal/routes/routes_test.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/responses.go`
- Test: `internal/api/responses_test.go`

**Interfaces:**
- Consumes: Task 1 `Record.Response` and `Store.GetByID`.
- Produces: `ResponsesRetrievePath = "/v1/responses/{response_id}"`, `handleResponse`, and exact non-streaming Response persistence.

- [ ] **Step 1: Write failing route metadata tests**

Add to `internal/routes/routes_test.go`:

```go
func TestRetrieveResponseRoute(t *testing.T) {
	path := "/v1/responses/resp_123"
	if got := NormalizePath(path); got != ResponsesRetrievePath {
		t.Fatalf("NormalizePath = %q", got)
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		if allowed, known := MethodAllowed(path, method); !known || !allowed {
			t.Fatalf("%s known=%v allowed=%v", method, known, allowed)
		}
	}
	if allowed, known := MethodAllowed(path, http.MethodDelete); !known || allowed {
		t.Fatalf("DELETE known=%v allowed=%v", known, allowed)
	}
	if allow, ok := AllowHeader(path); !ok || allow != "GET, HEAD" {
		t.Fatalf("AllowHeader = %q, %v", allow, ok)
	}
	if IsPublicPath(path) {
		t.Fatal("retrieve response route must require authentication")
	}
}

func TestRetrieveResponseRouteRejectsInvalidShapes(t *testing.T) {
	for _, path := range []string{"/v1/responses/", "/v1/responses/a/b"} {
		if got := NormalizePath(path); got != UnknownPathLabel {
			t.Fatalf("NormalizePath(%q) = %q", path, got)
		}
	}
}
```

- [ ] **Step 2: Run route tests and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/routes -run RetrieveResponseRoute -count=1"
```

Expected: build failure because `ResponsesRetrievePath` is undefined.

- [ ] **Step 3: Register and canonicalize the dynamic route**

In `internal/routes/routes.go`, add:

```go
ResponsesRetrievePath = "/v1/responses/{response_id}"
```

and:

```go
{Path: ResponsesRetrievePath, Methods: []string{http.MethodGet, http.MethodHead}},
```

Extract the current single-segment matching rule into a helper and use it for both dynamic resources:

```go
func canonicalPath(path string) (string, bool) {
	if _, ok := knownPaths[path]; ok {
		return path, true
	}
	for _, dynamic := range []struct{ prefix, canonical string }{
		{ModelsPath + "/", ModelsRetrievePath},
		{ResponsesPath + "/", ResponsesRetrievePath},
	} {
		if value, ok := singlePathSegment(path, dynamic.prefix); ok {
			_ = value
			return dynamic.canonical, true
		}
	}
	return "", false
}

func singlePathSegment(path, prefix string) (string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	value := strings.TrimPrefix(path, prefix)
	return value, value != "" && !strings.Contains(value, "/")
}
```

- [ ] **Step 4: Run route tests and verify GREEN**

Run `go test ./internal/routes -count=1` in the required WSL environment. Expected: PASS.

- [ ] **Step 5: Write failing non-streaming API tests**

Add `reflect` to the test imports. Add tests to `internal/api/responses_test.go` that create a response through the existing `doResponsesJSON` helper, then retrieve it:

```go
func TestRetrieveStoredResponse(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello","store":true}`, true)
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created response: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/responses/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var retrieved compat.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &retrieved); err != nil {
		t.Fatalf("decode retrieved response: %v", err)
	}
	if !reflect.DeepEqual(retrieved, created) {
		t.Fatalf("retrieved=%#v created=%#v", retrieved, created)
	}
}
```

Add a helper and table-driven 404 assertions:

```go
func retrieveResponse(handler http.Handler, id, key, method string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/v1/responses/"+id, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func assertResponseNotFound(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var got compat.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Type != "invalid_request_error" || got.Error.Param == nil || *got.Error.Param != "response_id" {
		t.Fatalf("error=%#v", got)
	}
}
```

Create these exact fixtures before calling `assertResponseNotFound`: `resp_missing` against an enabled store; a response ID returned from `store:false`; any ID against `newTestHandler(fake.New())`, whose store is disabled; and an ID created as `alpha-secret` through `newResponseStateIsolationHandler` but retrieved with `beta-secret`. Add these method checks:

```go
head := retrieveResponse(handler, created.ID, testAPIKey, http.MethodHead)
if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Type") != "application/json" {
	t.Fatalf("HEAD status=%d content-type=%q body=%q", head.Code, head.Header().Get("Content-Type"), head.Body.String())
}
unauthenticated := retrieveResponse(handler, created.ID, "", http.MethodGet)
assertError(t, unauthenticated, http.StatusUnauthorized, "authentication_error")
for _, method := range []string{http.MethodPost, http.MethodDelete} {
	rr := retrieveResponse(handler, created.ID, testAPIKey, method)
	assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
	if rr.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("%s Allow=%q", method, rr.Header().Get("Allow"))
	}
}
```

- [ ] **Step 6: Run API tests and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/api -run 'RetrieveStoredResponse|RetrieveResponse' -count=1"
```

Expected: route registration or 404 failures because no retrieve handler and no Response snapshot exist.

- [ ] **Step 7: Persist the final non-streaming Response**

In `handleResponses`, set the final store flag before marshaling and storing:

```go
shouldStore := req.Store == nil || *req.Store
willStore := shouldStore && s.responseStore != nil && s.responseStore.Enabled()
response.Store = willStore
if willStore {
	payload, err := json.Marshal(response)
	if err != nil {
		s.writeAuditedError(w, r, routes.ResponsesPath, externalModel, compat.ServerError(http.StatusInternalServerError, "failed to store response state"))
		return
	}
	transcript := append(append(append([]compat.ChatMessage(nil), history...), currentMessages...), chatResp.Choices[0].Message)
	err = s.responseStore.Put(responsestore.Record{
		ID: response.ID, Client: clientFromContext(r.Context()), Model: externalModel,
		Transcript: transcript, Response: payload,
	})
	if err != nil {
		if errors.Is(err, responsestore.ErrContextTooLarge) {
			s.writeAuditedError(w, r, routes.ResponsesPath, externalModel, compat.InvalidRequest("response context is too large", "previous_response_id"))
			return
		}
		s.writeAuditedError(w, r, routes.ResponsesPath, externalModel, compat.ServerError(http.StatusInternalServerError, "failed to store response state"))
		return
	}
}
```

Remove the old post-`Put` `response.Store = true`; the saved snapshot and returned object must already contain `store:true`.

- [ ] **Step 8: Register and implement the retrieve handler**

Add to `Server.routeHandlers()`:

```go
routes.ResponsesRetrievePath: s.handleResponse,
```

Add to `internal/api/responses.go`:

```go
func (s *Server) handleResponse(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("response_id")
	if s.responseStore == nil || !s.responseStore.Enabled() {
		s.writeError(w, r, responseNotFound())
		return
	}
	record, _, ok := s.responseStore.GetByID(id, clientFromContext(r.Context()))
	if !ok || len(record.Response) == 0 {
		s.writeError(w, r, responseNotFound())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(record.Response)
}

func responseNotFound() *compat.Error {
	param := "response_id"
	return compat.NewError(http.StatusNotFound, "invalid_request_error", "response not found", &param)
}
```

- [ ] **Step 9: Run API and repository tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/api ./internal/routes ./internal/responsestore -count=1 && go test ./..."
```

Expected: PASS, including all existing continuation tests.

- [ ] **Step 10: Commit non-streaming retrieval**

```bash
git add internal/routes/routes.go internal/routes/routes_test.go internal/api/server.go internal/api/responses.go internal/api/responses_test.go
git commit -m "feat: retrieve stored responses"
```

### Task 3: Persist and retrieve completed streaming Responses

**Files:**
- Modify: `internal/api/responses.go`
- Test: `internal/api/responses_test.go`

**Interfaces:**
- Consumes: Task 1 `Record.Response` and Task 2 retrieve handler.
- Produces: a stored snapshot equal to the Response in the terminal `response.completed` SSE event.

- [ ] **Step 1: Write the failing stream retrieval test**

Use the existing streaming Responses fake and SSE parsing helpers. Capture the `response.completed` event, extract its response, then GET it:

```go
func TestRetrieveStoredStreamingResponse(t *testing.T) {
	p := &responseStateStreamProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	stream := doResponsesJSON(handler, `{"model":"test-model","input":"hello","stream":true,"store":true}`, true)
	completed := completedResponseFromSSE(t, stream.Body.String())

	req := httptest.NewRequest(http.MethodGet, "/v1/responses/"+completed.ID, nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var retrieved compat.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &retrieved); err != nil {
		t.Fatalf("decode retrieved response: %v", err)
	}
	if !reflect.DeepEqual(retrieved, completed) {
		t.Fatalf("retrieved=%#v completed=%#v", retrieved, completed)
	}
}
```

Replace the existing `completedResponseID` parser with a reusable value parser and keep existing callers working:

```go
func completedResponseFromSSE(t *testing.T, stream string) compat.Response {
	t.Helper()
	for _, block := range strings.Split(stream, "\n\n") {
		if !strings.HasPrefix(block, "event: response.completed\n") {
			continue
		}
		line := strings.TrimPrefix(strings.SplitN(block, "\n", 2)[1], "data: ")
		var event struct {
			Response compat.Response `json:"response"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		return event.Response
	}
	t.Fatalf("missing response.completed: %s", stream)
	return compat.Response{}
}

func completedResponseID(t *testing.T, stream string) string {
	return completedResponseFromSSE(t, stream).ID
}
```

- [ ] **Step 2: Run the stream test and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/api -run TestRetrieveStoredStreamingResponse -count=1"
```

Expected: GET returns 404 because the stream record has no Response payload.

- [ ] **Step 3: Save the completed stream Response JSON**

Immediately after successfully emitting `response.completed`, encode the same `completed` value and include it in the record:

```go
if willStore {
	payload, err := json.Marshal(&completed)
	if err != nil {
		s.logger.Warn("failed to encode completed response stream", "error", err)
	} else {
		assistant := streamAssistantMessage(text, textStarted, functionOrder)
		transcript := append(append(append([]compat.ChatMessage(nil), history...), currentMessages...), assistant)
		if err := s.responseStore.Put(responsestore.Record{
			ID: responseID, Client: clientFromContext(r.Context()), Model: externalModel,
			Transcript: transcript, Response: payload,
		}); err != nil {
			s.logger.Warn("failed to store completed response stream", "error", err)
		}
	}
}
```

Replace the old transcript-only stream `Put`; do not create a record when marshaling fails.

- [ ] **Step 4: Run stream, Responses, and race tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/api -run 'Responses|RetrieveStoredStreamingResponse' -count=1 && go test -race ./internal/api ./internal/responsestore"
```

Expected: PASS.

- [ ] **Step 5: Commit streaming persistence**

```bash
git add internal/api/responses.go internal/api/responses_test.go
git commit -m "feat: retain completed streaming responses"
```

### Task 4: Document the compatibility contract and verify

**Files:**
- Create: `tasks/164-retrieve-response.md`
- Modify: `openai-compatible-proxy-spec.md`
- Modify: `architecture/overview.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: Tasks 1-3 completed endpoint behavior.
- Produces: the public contract and completed Task 164 record.

- [ ] **Step 1: Create the Task 164 record**

Create `tasks/164-retrieve-response.md`:

```markdown
# Task 164 - 查询已保存的 Response

## 状态

Done.

## 背景

网关可以保存 Responses API 的对话上下文，但调用方还不能按 response ID 读取创建时的完整响应。

## 变更

- 新增需要鉴权的 `GET` 和 `HEAD /v1/responses/{response_id}`。
- 同时保存 continuation transcript 和最终 Response JSON，并纳入现有 TTL、LRU 和 byte 限制。
- 对未知、未保存、过期、淘汰和其他 client 的 ID 返回统一 404。
- 对具体 response ID 路径进行日志和 metrics 规范化，读取不调用 provider。

## 验收

- 普通及流式 `store: true` Response 均可读取，字段和值与创建时一致。
- `store: false`、store 禁用和跨 client 读取返回 `404 invalid_request_error`。
- WSL `make verify` 通过。
```

- [ ] **Step 2: Update the public spec and architecture**

In `openai-compatible-proxy-spec.md`, add a Responses retrieval section documenting authentication, GET/HEAD, complete saved JSON, process-local TTL/LRU behavior, uniform 404 cases, and the absence of provider calls. State explicitly that delete, cancel, input items, `include`, and durable persistence remain unsupported.

In `architecture/overview.md`, update the Responses API and in-memory state sections: each record contains transcript plus final Response JSON; continuation uses model-aware `Get`, retrieval uses client-aware `GetByID`; both share TTL/LRU/capacity accounting.

- [ ] **Step 3: Update README and changelog**

Add `GET /v1/responses/{response_id}` to README's supported endpoints and document that only successfully completed stored Responses are retrievable. Under `CHANGELOG.md` `Unreleased > Added`, add:

```markdown
- Added authenticated retrieval of completed, locally stored Responses through `GET` and `HEAD /v1/responses/{response_id}` with client isolation and normalized observability paths.
```

- [ ] **Step 4: Check contract consistency**

Run:

```powershell
rg -n "responses/\{response_id\}|Response JSON|response_id" README.md openai-compatible-proxy-spec.md architecture/overview.md tasks/164-retrieve-response.md CHANGELOG.md
```

Expected: the route, storage limits, client isolation, 404 behavior, and non-goals appear consistently in the relevant documents.

- [ ] **Step 5: Run final verification in WSL**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "make verify"
```

Expected: line-ending checks, formatting, unit tests, race tests, and vet all pass.

- [ ] **Step 6: Review the final diff**

Run:

```powershell
git diff --check
git status --short
git diff --stat
```

Expected: no whitespace errors and only Task 164 code, tests, design/plan, and contract documentation are changed.

- [ ] **Step 7: Commit documentation**

```bash
git add tasks/164-retrieve-response.md openai-compatible-proxy-spec.md architecture/overview.md README.md CHANGELOG.md
git commit -m "docs: document response retrieval"
```

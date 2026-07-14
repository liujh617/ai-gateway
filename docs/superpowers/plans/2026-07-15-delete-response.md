# Delete Stored Response Endpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add authenticated `DELETE /v1/responses/{response_id}` support that atomically removes one locally stored Response and returns the OpenAI-compatible deletion object.

**Architecture:** Add a client-aware atomic delete operation to `responsestore`, separating structural removal from eviction accounting so explicit deletion does not look like TTL/capacity eviction. Extend the existing Response instance route to GET/HEAD/DELETE, using a methodless Go ServeMux pattern only for routes with multiple non-HEAD methods while retaining the shared method-validation middleware.

**Tech Stack:** Go 1.22 standard library (`net/http`, `encoding/json`), existing `compat`, `responsestore`, `routes`, middleware, and API packages; WSL `Ubuntu-24.04` for all verification.

## Global Constraints

- Prefer the Go standard library and add no dependency.
- Delete only the requested Response; never cascade to descendant Responses.
- Enforce existing Bearer authentication and gateway-client isolation.
- Unknown, expired, evicted, disabled, already-deleted, and cross-client IDs return the same `404 invalid_request_error` with `param: response_id`.
- Explicit deletion reduces entry/byte state but never increments expired/capacity eviction metrics.
- Do not call providers, inspect provider health, or create model invocation audit events for deletion.
- Normalize concrete IDs to `/v1/responses/{response_id}` in logs and metrics.
- Keep existing GET, HEAD, creation, continuation, TTL, LRU, miss, and eviction behavior intact.
- Update `openai-compatible-proxy-spec.md`, `architecture/overview.md`, README, changelog, Task 165, and the stale endpoint list in `docs/trae-deepseek.md`.
- Run final verification in WSL `Ubuntu-24.04` from `/mnt/e/code/ai-gateway`.

---

### Task 1: Add atomic client-aware store deletion

**Files:**
- Modify: `internal/responsestore/store.go`
- Test: `internal/responsestore/store_test.go`

**Interfaces:**
- Consumes: existing `MissReason`, `entry`, TTL, LRU, byte accounting, miss counters, and eviction counters.
- Produces: `func (s *Store) DeleteByID(id, client string) (MissReason, bool)` and an internal `removeEntryLocked(*entry)` primitive.

- [ ] **Step 1: Write failing deletion and accounting tests**

Add to `internal/responsestore/store_test.go`:

```go
func TestDeleteByIDRemovesRecordWithoutEviction(t *testing.T) {
	store := newTestStore(nil, nil)
	record := Record{
		ID: "resp_1", Client: "alpha", Model: "test-model",
		Transcript: []compat.ChatMessage{message("user", "hello")},
		Response:   json.RawMessage(`{"id":"resp_1"}`),
	}
	if err := store.Put(record); err != nil {
		t.Fatalf("Put: %v", err)
	}
	before := store.Snapshot()

	reason, ok := store.DeleteByID("resp_1", "alpha")
	if !ok || reason != "" {
		t.Fatalf("DeleteByID ok=%v reason=%q", ok, reason)
	}
	if _, reason, ok := store.GetByID("resp_1", "alpha"); ok || reason != MissNotFound {
		t.Fatalf("GetByID after delete ok=%v reason=%q", ok, reason)
	}
	after := store.Snapshot()
	if before.Entries != 1 || before.Bytes <= 0 || after.Entries != 0 || after.Bytes != 0 {
		t.Fatalf("before=%+v after=%+v", before, after)
	}
	if after.Evictions[EvictionExpired] != 0 || after.Evictions[EvictionCapacity] != 0 {
		t.Fatalf("explicit delete counted as eviction: %+v", after.Evictions)
	}
}

func TestDeleteByIDEnforcesClient(t *testing.T) {
	store := newTestStore(nil, nil)
	if err := store.Put(Record{ID: "resp_1", Client: "alpha", Model: "test-model", Response: json.RawMessage(`{"id":"resp_1"}`)}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if reason, ok := store.DeleteByID("resp_1", "beta"); ok || reason != MissClient {
		t.Fatalf("cross-client delete ok=%v reason=%q", ok, reason)
	}
	if _, _, ok := store.GetByID("resp_1", "alpha"); !ok {
		t.Fatal("cross-client delete removed the record")
	}
}
```

Add expiry and repeated-delete coverage:

```go
func TestDeleteByIDMissesAndExpiry(t *testing.T) {
	clock := &fakeClock{now: time.Unix(100, 0)}
	store := newTestStore(clock, func(c *Config) { c.TTL = time.Minute })
	if err := store.Put(Record{ID: "resp_1", Client: "alpha", Model: "test-model", Response: json.RawMessage(`{"id":"resp_1"}`)}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if reason, ok := store.DeleteByID("missing", "alpha"); ok || reason != MissNotFound {
		t.Fatalf("missing delete ok=%v reason=%q", ok, reason)
	}
	clock.Advance(time.Minute)
	if reason, ok := store.DeleteByID("resp_1", "alpha"); ok || reason != MissExpired {
		t.Fatalf("expired delete ok=%v reason=%q", ok, reason)
	}
	stats := store.Snapshot()
	if stats.Evictions[EvictionExpired] != 1 || stats.Misses[MissExpired] != 1 {
		t.Fatalf("stats=%+v", stats)
	}
	if reason, ok := store.DeleteByID("resp_1", "alpha"); ok || reason != MissNotFound {
		t.Fatalf("repeated delete ok=%v reason=%q", ok, reason)
	}
}
```

Add concurrent access coverage:

```go
func TestConcurrentDeleteByID(t *testing.T) {
	store := newTestStore(nil, func(c *Config) { c.MaxEntries = 100 })
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("resp_%d", i)
		if err := store.Put(Record{ID: id, Client: "alpha", Model: "test-model", Response: json.RawMessage(`{"object":"response"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("resp_%d", i)
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _, _ = store.GetByID(id, "alpha")
		}()
		go func() {
			defer wg.Done()
			if reason, ok := store.DeleteByID(id, "alpha"); !ok {
				t.Errorf("DeleteByID(%s) reason=%q", id, reason)
			}
		}()
	}
	wg.Wait()
	if stats := store.Snapshot(); stats.Entries != 0 || stats.Bytes != 0 {
		t.Fatalf("stats=%+v", stats)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/task-165-delete-response -- bash -lc "go test ./internal/responsestore -run DeleteByID -count=1"
```

Expected: build failure because `Store.DeleteByID` is undefined.

- [ ] **Step 3: Implement atomic deletion and separate removal accounting**

Add to `internal/responsestore/store.go` after `GetByID`:

```go
func (s *Store) DeleteByID(id, client string) (MissReason, bool) {
	if s == nil || !s.config.enabled() {
		return MissNotFound, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok {
		s.misses[MissNotFound]++
		return MissNotFound, false
	}
	if !s.clock.Now().Before(e.expiresAt) {
		s.removeLocked(e, EvictionExpired)
		s.misses[MissExpired]++
		return MissExpired, false
	}
	if e.record.Client != client {
		s.misses[MissClient]++
		return MissClient, false
	}
	s.removeEntryLocked(e)
	return "", true
}
```

Refactor the existing removal code to exactly this form:

```go
func (s *Store) removeLocked(e *entry, reason EvictionReason) {
	s.removeEntryLocked(e)
	s.evictions[reason]++
}

func (s *Store) removeEntryLocked(e *entry) {
	delete(s.entries, e.record.ID)
	s.lru.Remove(e.lru)
	s.bytes -= e.bytes
}
```

- [ ] **Step 4: Run all store tests and race coverage**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/task-165-delete-response -- bash -lc "gofmt -w internal/responsestore/store.go internal/responsestore/store_test.go && go test ./internal/responsestore -count=1 && go test -race ./internal/responsestore"
```

Expected: PASS with no race reports.

- [ ] **Step 5: Commit the store deletion primitive**

```bash
git add internal/responsestore/store.go internal/responsestore/store_test.go
git commit -m "feat: delete stored response state"
```

### Task 2: Extend route registration and expose DELETE

**Files:**
- Modify: `internal/routes/routes.go`
- Test: `internal/routes/routes_test.go`
- Modify: `internal/compat/responses.go`
- Test: `internal/compat/responses_test.go`
- Modify: `internal/api/responses.go`
- Test: `internal/api/responses_test.go`

**Interfaces:**
- Consumes: Task 1 `Store.DeleteByID` and existing `responseNotFound`.
- Produces: `compat.DeletedResponse`, methodless multi-method registration, and `DELETE /v1/responses/{response_id}`.

- [ ] **Step 1: Write failing route registration tests**

Update `TestRetrieveResponseRoute` in `internal/routes/routes_test.go` to require DELETE and the stable Allow header:

```go
for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodDelete} {
	if allowed, known := MethodAllowed(path, method); !known || !allowed {
		t.Fatalf("%s known=%v allowed=%v", method, known, allowed)
	}
}
if allow, ok := AllowHeader(path); !ok || allow != "GET, HEAD, DELETE" {
	t.Fatalf("AllowHeader = %q, %v", allow, ok)
}
```

Replace the existing assertion that DELETE is rejected with a PUT assertion:

```go
if allowed, known := MethodAllowed(path, http.MethodPut); !known || allowed {
	t.Fatalf("PUT known=%v allowed=%v", known, allowed)
}
```

Extend `TestRegistrationPattern`:

```go
responses := Route{Path: ResponsesRetrievePath, Methods: []string{http.MethodGet, http.MethodHead, http.MethodDelete}}
if got := responses.RegistrationPattern(); got != ResponsesRetrievePath {
	t.Fatalf("responses pattern = %q", got)
}
```

- [ ] **Step 2: Run route tests and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/task-165-delete-response -- bash -lc "go test ./internal/routes -run 'RetrieveResponseRoute|RegistrationPattern' -count=1"
```

Expected: failures because DELETE is not allowed and the registration pattern still contains `GET `.

- [ ] **Step 3: Implement multi-method route registration**

Change the Response route definition:

```go
{Path: ResponsesRetrievePath, Methods: []string{http.MethodGet, http.MethodHead, http.MethodDelete}},
```

Replace `RegistrationPattern` with:

```go
func (r Route) RegistrationPattern() string {
	method := r.registrationMethod()
	for _, item := range r.Methods {
		if item != http.MethodHead && item != method {
			return r.Path
		}
	}
	return Pattern(method, r.Path)
}
```

Keep `registrationMethod` unchanged so all single-primary-method routes preserve their existing registration patterns.

- [ ] **Step 4: Run all route tests and verify GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/task-165-delete-response -- bash -lc "gofmt -w internal/routes/routes.go internal/routes/routes_test.go && go test ./internal/routes -count=1"
```

Expected: PASS.

- [ ] **Step 5: Write the failing compat response-shape test**

Add to `internal/compat/responses_test.go`:

```go
func TestDeletedResponseJSON(t *testing.T) {
	payload, err := json.Marshal(DeletedResponse{ID: "resp_123", Object: "response", Deleted: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(payload), `{"id":"resp_123","object":"response","deleted":true}`; got != want {
		t.Fatalf("payload=%s want=%s", got, want)
	}
}
```

- [ ] **Step 6: Run the compat test and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/task-165-delete-response -- bash -lc "go test ./internal/compat -run TestDeletedResponseJSON -count=1"
```

Expected: build failure because `DeletedResponse` is undefined.

- [ ] **Step 7: Add the compat deletion type**

Add immediately after `Response` in `internal/compat/responses.go`:

```go
type DeletedResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}
```

- [ ] **Step 8: Run compat tests and verify GREEN**

Run `gofmt -w internal/compat/responses.go internal/compat/responses_test.go` and `go test ./internal/compat -count=1` in the required WSL worktree. Expected: PASS.

- [ ] **Step 9: Write failing API success, lifecycle, and provider-isolation tests**

Add to `internal/api/responses_test.go`:

```go
func TestDeleteStoredResponse(t *testing.T) {
	p := &responseStateProvider{}
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(p, api.Options{ResponseStore: store})
	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	rr := retrieveResponse(handler, created.ID, testAPIKey, http.MethodDelete)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type=%q", got)
	}
	var deleted compat.DeletedResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &deleted); err != nil {
		t.Fatal(err)
	}
	if deleted.ID != created.ID || deleted.Object != "response" || !deleted.Deleted {
		t.Fatalf("deleted=%#v", deleted)
	}
	assertResponseNotFound(t, retrieveResponse(handler, created.ID, testAPIKey, http.MethodGet))
	if len(p.requests) != 1 {
		t.Fatalf("delete called provider: requests=%d", len(p.requests))
	}
}

func TestDeletedResponseCannotBeContinued(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if rr := retrieveResponse(handler, created.ID, testAPIKey, http.MethodDelete); rr.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", rr.Code, rr.Body.String())
	}
	continued := doResponsesJSON(handler, `{"model":"test-model","input":"again","previous_response_id":"`+created.ID+`"}`, true)
	assertError(t, continued, http.StatusNotFound, "invalid_request_error")
}
```

Add non-cascading coverage:

```go
func TestDeleteResponseDoesNotCascadeToDescendant(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	firstRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"first"}`, true)
	var first compat.Response
	if err := json.Unmarshal(firstRecorder.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	secondRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"second","previous_response_id":"`+first.ID+`"}`, true)
	var second compat.Response
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if rr := retrieveResponse(handler, first.ID, testAPIKey, http.MethodDelete); rr.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr := retrieveResponse(handler, second.ID, testAPIKey, http.MethodGet); rr.Code != http.StatusOK {
		t.Fatalf("descendant GET status=%d body=%s", rr.Code, rr.Body.String())
	}
	continued := doResponsesJSON(handler, `{"model":"test-model","input":"third","previous_response_id":"`+second.ID+`"}`, true)
	if continued.Code != http.StatusOK {
		t.Fatalf("descendant continuation status=%d body=%s", continued.Code, continued.Body.String())
	}
}
```

- [ ] **Step 10: Write failing API error-isolation tests**

Add:

```go
func TestDeleteResponseNotFoundCases(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	assertResponseNotFound(t, retrieveResponse(handler, "resp_missing", testAPIKey, http.MethodDelete))
	assertResponseNotFound(t, retrieveResponse(newTestHandler(fake.New()), "resp_missing", testAPIKey, http.MethodDelete))

	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if rr := retrieveResponse(handler, created.ID, testAPIKey, http.MethodDelete); rr.Code != http.StatusOK {
		t.Fatalf("first delete status=%d body=%s", rr.Code, rr.Body.String())
	}
	assertResponseNotFound(t, retrieveResponse(handler, created.ID, testAPIKey, http.MethodDelete))
}

func TestDeleteResponseIsClientIsolated(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, nil)
	handler := newResponseStateIsolationHandler(&responseStateProvider{}, store)
	createdRecorder := doResponsesJSONWithKey(handler, `{"model":"test-model","input":"hello"}`, "alpha-secret")
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	assertResponseNotFound(t, retrieveResponse(handler, created.ID, "beta-secret", http.MethodDelete))
	if rr := retrieveResponse(handler, created.ID, "alpha-secret", http.MethodGet); rr.Code != http.StatusOK {
		t.Fatalf("owner GET status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDeleteResponseRequiresAuth(t *testing.T) {
	rr := retrieveResponse(newTestHandler(fake.New()), "resp_missing", "", http.MethodDelete)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}
```

Replace `TestRetrieveResponseMethodNotAllowed` with:

```go
func TestRetrieveResponseMethodNotAllowed(t *testing.T) {
	handler := newTestHandler(fake.New())
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		rr := retrieveResponse(handler, "resp_1", testAPIKey, method)
		assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
		if got := rr.Header().Get("Allow"); got != "GET, HEAD, DELETE" {
			t.Fatalf("%s Allow=%q", method, got)
		}
	}
}
```

Add an API expiry clock and test:

```go
type responseStoreClock struct{ now time.Time }

func (c *responseStoreClock) Now() time.Time { return c.now }

func TestDeleteExpiredResponseReturnsNotFound(t *testing.T) {
	clock := &responseStoreClock{now: time.Unix(100, 0)}
	store := responsestore.New(responsestore.Config{TTL: time.Minute, MaxEntries: 10, MaxContextBytes: 1 << 20, MaxTotalBytes: 2 << 20}, clock)
	handler := newTestHandlerWithOptions(&responseStateProvider{}, api.Options{ResponseStore: store})
	createdRecorder := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	var created compat.Response
	if err := json.Unmarshal(createdRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(time.Minute)
	assertResponseNotFound(t, retrieveResponse(handler, created.ID, testAPIKey, http.MethodDelete))
}
```

- [ ] **Step 11: Run API tests and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/task-165-delete-response -- bash -lc "go test ./internal/api -run 'DeleteResponse|DeleteStoredResponse|DeletedResponse|RetrieveResponseMethodNotAllowed' -count=1"
```

Expected: DELETE requests reach the current read handler and fail with response decoding or incorrect status because deletion is not implemented.

- [ ] **Step 12: Implement DELETE dispatch and response**

Replace the beginning of `handleResponse` in `internal/api/responses.go` with method dispatch and keep the existing GET/HEAD body after it:

```go
func (s *Server) handleResponse(w http.ResponseWriter, r *http.Request) {
	responseID := r.PathValue("response_id")
	if r.Method == http.MethodDelete {
		s.deleteResponse(w, r, responseID)
		return
	}
	if s.responseStore == nil || !s.responseStore.Enabled() {
		s.writeError(w, r, responseNotFound())
		return
	}
	record, _, ok := s.responseStore.GetByID(responseID, clientFromContext(r.Context()))
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

func (s *Server) deleteResponse(w http.ResponseWriter, r *http.Request, responseID string) {
	if s.responseStore == nil || !s.responseStore.Enabled() {
		s.writeError(w, r, responseNotFound())
		return
	}
	if _, ok := s.responseStore.DeleteByID(responseID, clientFromContext(r.Context())); !ok {
		s.writeError(w, r, responseNotFound())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(compat.DeletedResponse{ID: responseID, Object: "response", Deleted: true})
}
```

- [ ] **Step 13: Run focused and full Go tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/task-165-delete-response -- bash -lc "gofmt -w internal/api/responses.go internal/api/responses_test.go && go test ./internal/api ./internal/routes ./internal/compat ./internal/responsestore -count=1 && go test ./..."
```

Expected: PASS; existing GET/HEAD, creation, streaming, and continuation tests remain green.

- [ ] **Step 14: Commit the HTTP deletion contract**

```bash
git add internal/routes/routes.go internal/routes/routes_test.go internal/compat/responses.go internal/compat/responses_test.go internal/api/responses.go internal/api/responses_test.go
git commit -m "feat: delete stored responses"
```

### Task 3: Document Task 165 and run final verification

**Files:**
- Create: `tasks/165-delete-response.md`
- Modify: `openai-compatible-proxy-spec.md`
- Modify: `architecture/overview.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/trae-deepseek.md`

**Interfaces:**
- Consumes: Tasks 1-2 complete DELETE behavior.
- Produces: synchronized public contract, architecture, user docs, changelog, and Task 165 record.

- [ ] **Step 1: Create the Task 165 record**

Create `tasks/165-delete-response.md` with:

```markdown
# Task 165 - 删除已保存的 Response

## 状态

Done.

## 背景

网关已经支持保存、续接和读取进程内 Response，但调用方无法主动删除不再需要的记录。

## 变更

- 新增需要鉴权的 `DELETE /v1/responses/{response_id}`。
- 按 gateway client 原子删除目标记录，并同步清理 entries、LRU 和 byte 统计。
- 主动删除不计入 expired/capacity eviction 指标。
- 删除只作用于目标 ID，不级联删除后续 Response，也不调用 provider。

## 验收

- 成功返回 OpenAI-compatible `{ "id", "object": "response", "deleted": true }`。
- 未知、过期、已删除、store 禁用和跨 client 删除返回统一 404。
- 删除后目标无法读取或续接，已有 descendant Response 保持可用。
- WSL `make verify` 通过。
```

- [ ] **Step 2: Update the compatibility spec and architecture**

In `openai-compatible-proxy-spec.md`, append this contract to the Response instance section:

```markdown
`DELETE /v1/responses/{response_id}` 删除当前 gateway client 的目标记录。成功返回 `200` 和 `{ "id": "resp_...", "object": "response", "deleted": true }`。删除后目标无法再通过 GET 读取或作为 `previous_response_id` 使用；已经基于它创建的后续 Response 是独立快照，不受影响。

未知、过期、淘汰、已删除、store 禁用和属于其他 client 的 ID 统一返回 `404 invalid_request_error`，`param` 为 `response_id`。删除只访问本地 store，不调用 provider；不支持的方法返回 `Allow: GET, HEAD, DELETE`。
```

Append this sentence to the `internal/responsestore` paragraph in `architecture/overview.md`:

```markdown
Store 还提供按 client 原子删除单条 Response 的能力；显式删除复用底层 map/LRU/byte 清理，但不计入 expired/capacity eviction，且不会级联到已经保存为独立快照的 descendant Responses。
```

- [ ] **Step 3: Update README, changelog, and stale integration guide**

Add this README endpoint entry immediately after GET Response retrieval:

```markdown
- `DELETE /v1/responses/{response_id}`（删除同一 gateway client 保存的目标 Response，不级联删除后续 Response）
```

Under `CHANGELOG.md` `Unreleased > Added`, add:

```markdown
- Added authenticated deletion of locally stored Responses through `DELETE /v1/responses/{response_id}`, with client isolation, non-cascading semantics, and correct store accounting.
```

Replace the stale endpoint sentence in `docs/trae-deepseek.md` with:

```markdown
- 当前网关第一阶段支持 `/v1/models`、`/v1/models/{model}`、`/v1/chat/completions`、`/v1/responses`、`/v1/responses/{response_id}` 和 `/v1/embeddings`；各路径支持的 HTTP method 和兼容子集见 [OpenAI-compatible Proxy Spec](../openai-compatible-proxy-spec.md)。客户端如果强制依赖其他接口，需要先补齐对应兼容层。
```

- [ ] **Step 4: Check documentation consistency**

Run:

```powershell
rg -n "DELETE /v1/responses/\{response_id\}|deleted.*true|不级联|DeleteByID" README.md openai-compatible-proxy-spec.md architecture/overview.md tasks/165-delete-response.md CHANGELOG.md docs/trae-deepseek.md
```

Expected: the public contract, architecture, task record, changelog, and user-facing endpoint list consistently describe deletion.

- [ ] **Step 5: Run final WSL verification**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/task-165-delete-response -- bash -lc "make verify"
```

Expected: line-ending checks, formatting, all unit tests, all race tests, and vet pass.

- [ ] **Step 6: Review the final diff**

Run:

```powershell
git diff --check
git status --short
git diff --stat
```

Expected: no whitespace errors; only Task 165 code, tests, task record, and contract/user documentation are changed.

- [ ] **Step 7: Commit documentation**

```bash
git add tasks/165-delete-response.md openai-compatible-proxy-spec.md architecture/overview.md README.md CHANGELOG.md docs/trae-deepseek.md
git commit -m "docs: document response deletion"
```

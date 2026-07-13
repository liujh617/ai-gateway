# Retrieve Model Endpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add authenticated `GET` and `HEAD /v1/models/{model}` endpoints that return locally configured model metadata while enforcing client model allowlists.

**Architecture:** Register a Go 1.22 ServeMux wildcard route and extend the shared route registry so concrete model paths map to one canonical observability label. Add a read-only `ModelRouter.Model` lookup, then keep authentication, allowlist filtering, errors, and JSON encoding in the API layer without contacting an upstream provider.

**Tech Stack:** Go 1.22 standard library (`net/http`, `encoding/json`), existing `compat`, `router`, `routes`, and API packages; WSL `Ubuntu-24.04` for verification.

## Global Constraints

- Prefer the Go standard library; add no dependency.
- Keep handler, compat, router, and provider boundaries intact.
- Do not contact providers or inspect provider health for model metadata.
- Unknown and client-hidden models must return the same stable `404 invalid_request_error` response.
- Normalize concrete model IDs to `/v1/models/{model}` in logs and metrics.
- Update `openai-compatible-proxy-spec.md`, `architecture/overview.md`, the related task document, and README/CHANGELOG.
- Run final verification in WSL `Ubuntu-24.04` from `/mnt/e/code/ai-gateway`.

---

### Task 1: Retrieve model route, lookup, and handler

**Files:**
- Modify: `internal/routes/routes.go`
- Test: `internal/routes/routes_test.go`

**Interfaces:**
- Consumes: existing `Route`, `NormalizePath`, `AllowedMethods`, `MethodAllowed`, `AllowHeader`, and `IsPublicPath` APIs.
- Produces: `ModelsRetrievePath = "/v1/models/{model}"` and concrete-path recognition for `/v1/models/<one-segment-id>`.

- [ ] **Step 1: Write failing route tests**

Add these tests to `internal/routes/routes_test.go`:

```go
func TestRetrieveModelRoute(t *testing.T) {
	path := "/v1/models/test-model"
	if got := NormalizePath(path); got != ModelsRetrievePath {
		t.Fatalf("NormalizePath = %q", got)
	}
	if allowed, known := MethodAllowed(path, http.MethodGet); !known || !allowed {
		t.Fatalf("GET known=%v allowed=%v", known, allowed)
	}
	if allowed, known := MethodAllowed(path, http.MethodHead); !known || !allowed {
		t.Fatalf("HEAD known=%v allowed=%v", known, allowed)
	}
	if allowed, known := MethodAllowed(path, http.MethodPost); !known || allowed {
		t.Fatalf("POST known=%v allowed=%v", known, allowed)
	}
	if allow, ok := AllowHeader(path); !ok || allow != "GET, HEAD" {
		t.Fatalf("AllowHeader = %q, %v", allow, ok)
	}
	if IsPublicPath(path) {
		t.Fatal("retrieve model route must require authentication")
	}
}

func TestRetrieveModelRouteRejectsInvalidShapes(t *testing.T) {
	for _, path := range []string{"/v1/models/", "/v1/models/a/b"} {
		if got := NormalizePath(path); got != UnknownPathLabel {
			t.Fatalf("NormalizePath(%q) = %q", path, got)
		}
		if _, known := AllowedMethods(path); known {
			t.Fatalf("AllowedMethods(%q) marked path known", path)
		}
	}
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/routes -run RetrieveModelRoute -count=1"
```

Expected: build failure because `ModelsRetrievePath` is undefined.

- [ ] **Step 3: Implement canonical dynamic-path matching**

In `internal/routes/routes.go`, add the constant and definition:

```go
ModelsRetrievePath = "/v1/models/{model}"
```

```go
{Path: ModelsRetrievePath, Methods: []string{http.MethodGet, http.MethodHead}},
```

Add one internal canonicalizer and route all metadata lookup functions through it:

```go
func canonicalPath(path string) (string, bool) {
	if _, ok := knownPaths[path]; ok {
		return path, true
	}
	const prefix = ModelsPath + "/"
	if strings.HasPrefix(path, prefix) {
		model := strings.TrimPrefix(path, prefix)
		if model != "" && !strings.Contains(model, "/") {
			return ModelsRetrievePath, true
		}
	}
	return "", false
}
```

Change `NormalizePath`, `AllowedMethods`, `MethodAllowed`, `AllowHeader`, and `IsPublicPath` to call `canonicalPath` before reading their existing maps. `NormalizePath` must return `UnknownPathLabel` when canonicalization fails; `IsPublicPath` must return false when it fails.

- [ ] **Step 4: Run route tests and verify GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/routes -count=1"
```

Expected: PASS.

- [ ] **Step 5: Continue without committing the incomplete registry**

Do not commit yet: adding the route definition intentionally makes `Server.routeHandlers()` incomplete until the handler in the following steps is added. Continue directly to the router and API tests so the task ends with a repository-wide green state.

#### Router metadata lookup and API handler

**Files:**
- Modify: `internal/router/model_router.go`
- Test: `internal/router/model_router_test.go`
- Modify: `internal/api/server.go`
- Test: `internal/api/chat_completions_test.go`

**Interfaces:**
- Consumes: Task 1 `routes.ModelsRetrievePath`, existing `modelAllowedForRequest`, `compat.Model`, and `compat.ModelNotFound`.
- Produces: `func (r *ModelRouter) Model(model string) (compat.Model, bool)` and the GET/HEAD handler.

- [ ] **Step 1: Write the failing router lookup test**

Add to `internal/router/model_router_test.go`:

```go
func TestModelReturnsConfiguredMetadata(t *testing.T) {
	modelRouter := router.NewModelRouter([]router.ModelRoute{{ExternalModel: "test-model"}})

	model, ok := modelRouter.Model("test-model")
	if !ok {
		t.Fatal("configured model was not found")
	}
	if model.ID != "test-model" || model.Object != "model" || model.Created != 0 || model.OwnedBy != "open-ai-gateway" {
		t.Fatalf("model = %#v", model)
	}
	if _, ok := modelRouter.Model("missing"); ok {
		t.Fatal("missing model was found")
	}
}
```

- [ ] **Step 2: Run the router test and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/router -run TestModelReturnsConfiguredMetadata -count=1"
```

Expected: build failure because `Model` is undefined.

- [ ] **Step 3: Implement the router lookup**

Add a shared metadata constructor and use it from both `Model` and `Models`:

```go
func (r *ModelRouter) Model(model string) (compat.Model, bool) {
	if r == nil {
		return compat.Model{}, false
	}
	if _, ok := r.routes[model]; !ok {
		return compat.Model{}, false
	}
	return modelMetadata(model), true
}

func modelMetadata(model string) compat.Model {
	return compat.Model{
		ID:      model,
		Object:  "model",
		Created: 0,
		OwnedBy: "open-ai-gateway",
	}
}
```

Replace the inline `compat.Model` literal in `Models()` with `modelMetadata(model)`.

- [ ] **Step 4: Run router tests and verify GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/router -count=1"
```

Expected: PASS.

- [ ] **Step 5: Write failing API behavior tests**

Add focused tests to `internal/api/chat_completions_test.go`. Use `newTestHandler(fake.New())` for normal, HEAD, auth, unknown-model, and method tests; use `newClientModelAccessTestHandler()` for allowlist tests.

```go
func TestRetrieveModelOK(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/models/test-model", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var model compat.Model
	if err := json.Unmarshal(rr.Body.Bytes(), &model); err != nil {
		t.Fatalf("decode model: %v", err)
	}
	if model.ID != "test-model" || model.Object != "model" || model.OwnedBy != "open-ai-gateway" {
		t.Fatalf("model = %#v", model)
	}
}

func TestRetrieveModelHeadOK(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodHead, "/v1/models/test-model", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || rr.Body.Len() != 0 {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}
}

func TestRetrieveModelRequiresAuth(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/models/test-model", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestRetrieveModelNotFound(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/models/missing", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestRetrieveModelRespectsClientAllowlist(t *testing.T) {
	handler, _ := newClientModelAccessTestHandler()
	for _, tc := range []struct {
		model string
		want  int
	}{{"test-model", http.StatusOK}, {"other-model", http.StatusNotFound}} {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/"+tc.model, nil)
		req.Header.Set("Authorization", "Bearer alpha-secret")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Fatalf("model %q status = %d, body = %s", tc.model, rr.Code, rr.Body.String())
		}
	}
}

func TestRetrieveModelMethodNotAllowed(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodPost, "/v1/models/test-model", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
	if got := rr.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("allow = %q", got)
	}
}
```

- [ ] **Step 6: Run API tests and verify RED**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/api -run RetrieveModel -count=1"
```

Expected: failures because no handler is registered for `ModelsRetrievePath`.

- [ ] **Step 7: Register and implement the API handler**

Add the handler entry to `routeHandlers()` in `internal/api/server.go`:

```go
routes.ModelsRetrievePath: s.handleModel,
```

Add the handler:

```go
func (s *Server) handleModel(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("model")
	if !s.modelAllowedForRequest(r, modelID) {
		s.writeError(w, r, compat.ModelNotFound(modelID))
		return
	}
	model, ok := s.router.Model(modelID)
	if !ok {
		s.writeError(w, r, compat.ModelNotFound(modelID))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(model)
}
```

- [ ] **Step 8: Run API tests and verify GREEN**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/api -run 'RetrieveModel|Models' -count=1"
```

Expected: PASS.

- [ ] **Step 9: Run all Go tests before committing**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./..."
```

Expected: PASS.

- [ ] **Step 10: Commit the endpoint implementation**

```bash
git add internal/router/model_router.go internal/router/model_router_test.go internal/api/server.go internal/api/chat_completions_test.go
git commit -m "feat: retrieve configured model metadata"
```

### Task 2: Public contract and task documentation

**Files:**
- Create: `tasks/163-retrieve-model.md`
- Modify: `openai-compatible-proxy-spec.md`
- Modify: `architecture/overview.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: the API behavior delivered by Task 1.
- Produces: a documented external contract and completed Task 163 record.

- [ ] **Step 1: Write the task record**

Create `tasks/163-retrieve-model.md` with:

```markdown
# Task 163 - 查询单个模型

## Status

Done.

## 背景

网关已经通过 `GET /v1/models` 列出当前 client 可见的模型，但尚未支持按模型 ID 查询单个已配置模型。

## 变更

- 新增需要鉴权的 `GET` 和 `HEAD /v1/models/{model}`。
- 执行 gateway client 模型白名单，并避免泄露隐藏模型是否存在。
- 为日志、metrics、方法处理和路由元数据规范化具体模型路径。
- 查询只读取本地配置，不调用或健康检查上游 provider。

## 验收

- 已知且可见的模型返回 OpenAI-compatible 模型元数据。
- 未知模型和 client 隐藏模型返回相同的 `404 invalid_request_error`。
- WSL `make verify` 通过。
```

- [ ] **Step 2: Update the external contract and architecture**

In `openai-compatible-proxy-spec.md`, add a `GET /v1/models/{model}` section after the model-list section documenting authentication, allowlist behavior, GET response shape, HEAD semantics, uniform 404 behavior, and no upstream call.

In `architecture/overview.md`, update HTTP API responsibilities to mention list/retrieve model metadata and state that retrieval reads static router configuration.

- [ ] **Step 3: Update README and changelog**

Add `GET /v1/models/{model}` to README's first-phase endpoint list. Under `CHANGELOG.md` `Unreleased > Added`, add:

```markdown
- Added authenticated single-model metadata retrieval through `GET` and `HEAD /v1/models/{model}`, including client allowlist enforcement and normalized observability paths.
```

- [ ] **Step 4: Check documentation consistency**

Run:

```powershell
rg -n "GET /v1/models/\{model\}|HEAD /v1/models/\{model\}" README.md openai-compatible-proxy-spec.md architecture/overview.md tasks/163-retrieve-model.md CHANGELOG.md
```

Expected: the new route appears in the public contract, task record, README, architecture, and changelog where applicable.

- [ ] **Step 5: Run final WSL verification**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "make verify"
```

Expected: line-ending checks, format, unit tests, race tests, and vet all pass.

- [ ] **Step 6: Review the final diff**

Run:

```powershell
git diff --check
git status --short
git diff --stat
```

Expected: no whitespace errors; only Task 163 code, tests, and documentation are changed.

- [ ] **Step 7: Commit documentation**

```bash
git add tasks/163-retrieve-model.md openai-compatible-proxy-spec.md architecture/overview.md README.md CHANGELOG.md
git commit -m "docs: document retrieve model endpoint"
```

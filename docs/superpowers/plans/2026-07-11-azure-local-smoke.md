# Azure OpenAI Local Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增加无需真实 Azure 凭据、可由 CI 执行的本地 Azure OpenAI 端到端 smoke，覆盖非流式 chat、SSE chat 和 embeddings。

**Architecture:** 一个仅供测试使用的 Go HTTP server 严格模拟三条 Azure deployment endpoint 并断言请求契约；Bash 脚本同时启动 fake upstream 与真实 gateway，通过临时配置和 curl 验证完整链路。`make release-check` 调用新 smoke，因此现有 GitHub/Gitee workflow 无需修改。

**Tech Stack:** Go 1.22 标准库、`net/http`、`httptest`、Bash、curl、Make、WSL `Ubuntu-24.04`。

## Global Constraints

- 不访问真实 Azure OpenAI endpoint，不读取真实 Azure API key。
- 不修改 Azure provider、router、handler 或其他生产行为。
- 不新增第三方依赖；fake server 只使用 Go 标准库。
- 文档使用中文并保留必要英文协议名和字段名。
- 最终验证必须在 WSL `Ubuntu-24.04` 中执行。
- CI 入口保持 `make release-check`，不直接修改 GitHub/Gitee workflow。

---

### Task 1: 以 TDD 实现严格的 Azure fake upstream

**Files:**
- Create: `internal/testupstream/azurefake/main_test.go`
- Create: `internal/testupstream/azurefake/main.go`

**Interfaces:**
- Consumes: 环境变量 `AZURE_FAKE_ADDR`，缺省值 `127.0.0.1:19090`。
- Produces: `newHandler() http.Handler`；本地 endpoint `GET /healthz`、Azure chat deployment endpoint 和 embeddings deployment endpoint。

- [ ] **Step 1: 创建 fake upstream 的失败测试**

创建 `internal/testupstream/azurefake/main_test.go`，包含以下测试辅助函数与表驱动覆盖：

```go
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestChatCompletion(t *testing.T) {
	rec := serveModelRequest(t, "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", "application/json", `{"model":"chat-deployment","messages":[{"role":"user","content":"hello"}]}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"object":"chat.completion"`) {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestStreamingChatCompletion(t *testing.T) {
	rec := serveModelRequest(t, "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", "text/event-stream", `{"model":"chat-deployment","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "data: [DONE]") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestEmbedding(t *testing.T) {
	rec := serveModelRequest(t, "/openai/deployments/embedding-deployment/embeddings?api-version=2024-02-15-preview", "application/json", `{"model":"embedding-deployment","input":"hello"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"object":"list"`) {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestRejectsInvalidAzureContract(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		apiKey        string
		authorization string
		contentType   string
		accept        string
		body          string
	}{
		{name: "api version", path: "/openai/deployments/chat-deployment/chat/completions?api-version=wrong", apiKey: "local-azure-test-key", contentType: "application/json", accept: "application/json", body: `{"model":"chat-deployment"}`},
		{name: "api key", path: "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", apiKey: "wrong", contentType: "application/json", accept: "application/json", body: `{"model":"chat-deployment"}`},
		{name: "authorization", path: "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", apiKey: "local-azure-test-key", authorization: "Bearer forbidden", contentType: "application/json", accept: "application/json", body: `{"model":"chat-deployment"}`},
		{name: "content type", path: "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", apiKey: "local-azure-test-key", contentType: "text/plain", accept: "application/json", body: `{"model":"chat-deployment"}`},
		{name: "accept", path: "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", apiKey: "local-azure-test-key", contentType: "application/json", accept: "text/plain", body: `{"model":"chat-deployment"}`},
		{name: "model", path: "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", apiKey: "local-azure-test-key", contentType: "application/json", accept: "application/json", body: `{"model":"wrong"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("api-key", tt.apiKey)
			req.Header.Set("Authorization", tt.authorization)
			req.Header.Set("Content-Type", tt.contentType)
			req.Header.Set("Accept", tt.accept)
			rec := httptest.NewRecorder()
			newHandler().ServeHTTP(rec, req)
			if rec.Code < 400 {
				t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func serveModelRequest(t *testing.T, path, accept, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("api-key", "local-azure-test-key")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, req)
	return rec
}
```

- [ ] **Step 2: 运行测试并确认因 handler 缺失而失败**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/testupstream/azurefake -count=1"
```

Expected: FAIL，错误包含 `undefined: newHandler`。

- [ ] **Step 3: 实现最小 fake upstream**

创建 `internal/testupstream/azurefake/main.go`：

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultAddr = "127.0.0.1:19090"
	apiVersion  = "2024-02-15-preview"
	testAPIKey  = "local-azure-test-key"
)

type modelRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

func main() {
	addr := strings.TrimSpace(os.Getenv("AZURE_FAKE_ADDR"))
	if addr == "" {
		addr = defaultAddr
	}
	server := &http.Server{Addr: addr, Handler: newHandler(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("azure fake upstream listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("POST /openai/deployments/chat-deployment/chat/completions", handleChat)
	mux.HandleFunc("POST /openai/deployments/embedding-deployment/embeddings", handleEmbedding)
	return mux
}

func validateCommon(w http.ResponseWriter, r *http.Request, wantAccept string) bool {
	if r.URL.Query().Get("api-version") != apiVersion {
		http.Error(w, "api-version mismatch", http.StatusBadRequest)
		return false
	}
	if r.Header.Get("api-key") != testAPIKey {
		http.Error(w, "api-key mismatch", http.StatusUnauthorized)
		return false
	}
	if r.Header.Get("Authorization") != "" {
		http.Error(w, "Authorization must be absent", http.StatusBadRequest)
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return false
	}
	if r.Header.Get("Accept") != wantAccept {
		http.Error(w, "Accept mismatch", http.StatusBadRequest)
		return false
	}
	return true
}

func decodeModel(w http.ResponseWriter, r *http.Request, want string) (modelRequest, bool) {
	var req modelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return modelRequest{}, false
	}
	if req.Model != want {
		http.Error(w, "model mismatch", http.StatusBadRequest)
		return modelRequest{}, false
	}
	return req, true
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	var preview modelRequest
	if err := json.NewDecoder(r.Body).Decode(&preview); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	wantAccept := "application/json"
	if preview.Stream {
		wantAccept = "text/event-stream"
	}
	if !validateCommon(w, r, wantAccept) {
		return
	}
	if preview.Model != "chat-deployment" {
		http.Error(w, "model mismatch", http.StatusBadRequest)
		return
	}
	if preview.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"chunk_azure_smoke\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"chat-deployment\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"id":"chatcmpl_azure_smoke","object":"chat.completion","created":1,"model":"chat-deployment","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
}

func handleEmbedding(w http.ResponseWriter, r *http.Request) {
	if !validateCommon(w, r, "application/json") {
		return
	}
	if _, ok := decodeModel(w, r, "embedding-deployment"); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"object":"list","model":"embedding-deployment","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"usage":{"prompt_tokens":1,"total_tokens":1}}`)
}
```

实施注意：`handleChat` 必须只 decode body 一次；`validateCommon` 不读取 body。`decodeModel` 只用于 embeddings。错误信息不得包含实际 header value。

- [ ] **Step 4: 格式化并运行 fake upstream 测试**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "gofmt -w internal/testupstream/azurefake && go test ./internal/testupstream/azurefake -count=1"
```

Expected: PASS，输出包含 `ok open-ai-gateway/internal/testupstream/azurefake`。

- [ ] **Step 5: 提交 fake upstream**

```powershell
git add internal/testupstream/azurefake/main.go internal/testupstream/azurefake/main_test.go
git commit -m "test: add local Azure fake upstream"
```

---

### Task 2: 编排 Azure smoke 并接入 release-check

**Files:**
- Create: `scripts/smoke-azure.sh`
- Modify: `Makefile`
- Modify: `docs/ci.md`
- Modify: `docs/local-verification.md`
- Create: `tasks/154-azure-local-smoke.md`

**Interfaces:**
- Consumes: `AZURE_FAKE_ADDR`、`GATEWAY_AZURE_SMOKE_ADDR` 可选覆盖；Task 1 的 fake upstream。
- Produces: `make smoke-azure`；成功标记 `smoke-azure-ok`；`release-check` 中的无凭据 Azure E2E 覆盖。

- [ ] **Step 1: 创建 Azure smoke 脚本**

创建 `scripts/smoke-azure.sh`，结构必须包含：

```bash
#!/usr/bin/env bash
set -euo pipefail

fake_addr="${AZURE_FAKE_ADDR:-127.0.0.1:19090}"
gateway_addr="${GATEWAY_AZURE_SMOKE_ADDR:-127.0.0.1:18083}"
gateway_key="azure-smoke-gateway-key"
tmpdir="$(mktemp -d)"
config="$tmpdir/config.json"
fake_log="$tmpdir/azure-fake.log"
gateway_log="$tmpdir/gateway.log"

cleanup() {
  status=$?
  if [ -n "${gateway_pid:-}" ]; then
    kill "$gateway_pid" 2>/dev/null || true
    wait "$gateway_pid" 2>/dev/null || true
  fi
  if [ -n "${fake_pid:-}" ]; then
    kill "$fake_pid" 2>/dev/null || true
    wait "$fake_pid" 2>/dev/null || true
  fi
  if [ "$status" -ne 0 ]; then
    echo "--- azure fake upstream log ---" >&2
    test ! -f "$fake_log" || sed 's/local-azure-test-key/[REDACTED]/g' "$fake_log" >&2
    echo "--- gateway log ---" >&2
    test ! -f "$gateway_log" || sed 's/local-azure-test-key/[REDACTED]/g' "$gateway_log" >&2
  fi
  rm -rf "$tmpdir"
  exit "$status"
}
trap cleanup EXIT

AZURE_FAKE_ADDR="$fake_addr" go run ./internal/testupstream/azurefake >"$fake_log" 2>&1 &
fake_pid=$!

for _ in $(seq 1 30); do
  if curl -fsS "http://$fake_addr/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://$fake_addr/healthz" | grep -q '"status":"ok"'

cat >"$config" <<JSON
{
  "addr": "$gateway_addr",
  "api_key": "$gateway_key",
  "providers": {
    "azure": {
      "type": "azure-openai",
      "base_url": "http://$fake_addr",
      "api_key": "local-azure-test-key",
      "api_version": "2024-02-15-preview"
    }
  },
  "models": {
    "azure-chat": {
      "provider": "azure",
      "upstream_model": "chat-deployment",
      "capabilities": ["chat"]
    },
    "azure-embedding": {
      "provider": "azure",
      "upstream_model": "embedding-deployment",
      "capabilities": ["embeddings"]
    }
  }
}
JSON

GATEWAY_CONFIG="$config" GATEWAY_ADDR= GATEWAY_API_KEY= GATEWAY_API_KEYS= \
  go run ./cmd/gateway >"$gateway_log" 2>&1 &
gateway_pid=$!

for _ in $(seq 1 30); do
  if curl -fsS "http://$gateway_addr/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://$gateway_addr/healthz" | grep -q '"status":"ok"'

models="$(curl -fsS "http://$gateway_addr/v1/models" -H "Authorization: Bearer $gateway_key")"
grep -q '"id":"azure-chat"' <<<"$models"
grep -q '"id":"azure-embedding"' <<<"$models"

curl -fsS "http://$gateway_addr/v1/chat/completions" \
  -H "Authorization: Bearer $gateway_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"azure-chat","messages":[{"role":"user","content":"hello"}]}' \
  | grep -q '"object":"chat.completion"'

stream="$(curl -fsS -N "http://$gateway_addr/v1/chat/completions" \
  -H "Authorization: Bearer $gateway_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"azure-chat","stream":true,"messages":[{"role":"user","content":"hello"}]}')"
grep -q '"object":"chat.completion.chunk"' <<<"$stream"
grep -q 'data: \[DONE\]' <<<"$stream"

curl -fsS "http://$gateway_addr/v1/embeddings" \
  -H "Authorization: Bearer $gateway_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"azure-embedding","input":"hello"}' \
  | grep -q '"object":"list"'

echo "smoke-azure-ok"
```

脚本必须保持 LF 换行。不要设置 executable bit，现有 Make target 通过 `bash` 调用脚本。

- [ ] **Step 2: 接入 Makefile 并先运行独立 smoke**

修改 `.PHONY`，加入 `smoke-azure`：

```make
.PHONY: fmt test race vet verify build run check-config check-config-examples smoke smoke-rate-limit smoke-azure smoke-deepseek smoke-deepseek-skip release-check docker-build docker-run
```

在 `smoke-rate-limit` 后新增：

```make
smoke-azure:
	bash scripts/smoke-azure.sh
```

将 release-check 改为：

```make
release-check: verify check-config check-config-examples build smoke smoke-rate-limit smoke-azure smoke-deepseek-skip
```

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "make smoke-azure"
```

Expected: exit code `0`，最后输出 `smoke-azure-ok`。

- [ ] **Step 3: 更新 CI 与本地验证文档**

在 `docs/ci.md` 的 `make release-check` 子命令列表中加入：

```markdown
- `make smoke-azure`
```

在 smoke 说明和 Notes 中明确：`make smoke-azure` 启动本地 Go fake Azure upstream，严格验证
deployment path、`api-version`、`api-key`、无 `Authorization`、chat、SSE 和 embeddings；不使用
真实凭据且不访问外网。

在 `docs/local-verification.md` 的 smoke 命令区域加入：

````markdown
Azure OpenAI 本地 fake upstream smoke：

```bash
make smoke-azure
```

该命令不需要 Azure 凭据，也不会访问外网。
````

- [ ] **Step 4: 新增 Task 154 文档**

创建 `tasks/154-azure-local-smoke.md`：

```markdown
# Task 154 - Azure OpenAI Local Smoke

## 状态

Done.

## 背景

Azure OpenAI provider 已有单元测试，但 CI 尚未通过真实 gateway 进程验证配置加载、模型路由、
deployment endpoint 和三类 API 的完整链路。

## 变更

- 新增 Go 标准库实现的本地 Azure fake upstream。
- 新增无需凭据的 `scripts/smoke-azure.sh`。
- 覆盖非流式 chat、SSE chat 和 embeddings。
- 严格验证 Azure path、`api-version`、`api-key`，并确认不发送 `Authorization`。
- 新增 `make smoke-azure` 并接入 `make release-check`。
- 同步 CI 与本地验证文档。

## 验收

- `go test ./internal/testupstream/azurefake -count=1`
- `make smoke-azure`
- `make release-check`
```

- [ ] **Step 5: 运行完整 WSL 验证并检查范围**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "make release-check"
git diff --check
git status --short
```

Expected:

- `make release-check` exit code `0`。
- 输出包含 `smoke-ok`、`smoke-rate-limit-ok`、`smoke-azure-ok` 和
  `smoke-deepseek-skip: DEEPSEEK_API_KEY is not set`。
- `git diff --check` 无输出。
- 变更只包含 fake upstream、Azure smoke 脚本、Makefile、两份文档和 Task 154；生产 gateway/provider
  Go 文件无变更。
- 若 WSL `gofmt -w` 造成 Windows 工作树纯换行符状态变化，确认
  `git diff --ignore-space-at-eol` 无语义差异后机械恢复，不纳入提交。

- [ ] **Step 6: 提交 smoke 与 CI 接入**

```powershell
git add scripts/smoke-azure.sh Makefile docs/ci.md docs/local-verification.md tasks/154-azure-local-smoke.md
git commit -m "test: add local Azure smoke coverage"
git status --short --branch
```

Expected: commit 成功，工作区干净。

# Stream Read Timeout Race Test Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用确定性的单元测试替换依赖毫秒级真实时间窗口的 SSE 读取超时测试，使 WSL race 验证稳定通过且不修改生产行为。

**Architecture:** `internal/provider/httpx` 负责 SSE body 读取与 transport timeout 归一化，因此在该包的测试中注入立即返回 timeout error 的 `io.ReadCloser`。OpenAI provider 继续覆盖建连阶段 timeout 和 provider 专属 SSE 行为，但删除重复的读取阶段时序测试。

**Tech Stack:** Go 标准库、`testing`、`io.ReadCloser`、`net.Error`、WSL `Ubuntu-24.04`、Go race detector。

## Global Constraints

- 只处理流式读取 timeout race 测试不稳定，不处理 `make verify` 格式化行为或其他问题。
- 不修改任何生产 Go 文件、公开 API、配置、兼容契约或架构文档。
- 优先使用 Go 标准库，不新增依赖。
- 最终验证必须在 WSL `Ubuntu-24.04` 中执行。
- 工作目录使用当前仓库实际 WSL 路径 `/mnt/e/code/ai-gateway`。

---

### Task 1: 将流式读取超时覆盖改为确定性分层测试

**Files:**
- Modify: `internal/provider/httpx/httpx_test.go:85`
- Modify: `internal/provider/openai/openai_test.go:175`
- Create: `tasks/153-stream-read-timeout-race-test-stability.md`

**Interfaces:**
- Consumes: `httpx.NewChatCompletionStream(body io.ReadCloser) *httpx.ChatCompletionStream` 和 `(*httpx.ChatCompletionStream).Next(context.Context) (*compat.ChatCompletionChunk, error)`。
- Produces: 测试专用 `timeoutReadCloser`，其 `Read` 返回现有 `timeoutError`，其 `Close` 返回 `nil`；不产生生产接口。

- [ ] **Step 1: 记录当前失败模式并确认工作区基线**

Run:

```powershell
git status --short --branch
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test -race ./internal/provider/openai -run TestStreamChatCompletionReadTimeoutIsDeadlineExceeded -count=100"
```

Expected:

- `git status` 只显示本计划和已确认 spec 带来的预期提交状态，不存在未解释的实现改动。
- 目标测试可能通过或复现 `StreamChatCompletion: upstream request timeout: context deadline exceeded`；由于问题是调度相关的 flaky test，不要求该命令每次必然失败。
- 无论是否复现，都以此前完整 `make verify` 在 `openai_test.go:198` 的失败作为已记录回归证据。

- [ ] **Step 2: 用确定性 reader 重写 `httpx` 读取超时测试**

在 `timeoutError` 方法之后新增测试 reader：

```go
type timeoutReadCloser struct{}

func (*timeoutReadCloser) Read([]byte) (int, error) { return 0, &timeoutError{} }
func (*timeoutReadCloser) Close() error             { return nil }
```

将 `TestStreamReadTimeoutIsDeadlineExceeded` 完整替换为：

```go
func TestStreamReadTimeoutIsDeadlineExceeded(t *testing.T) {
	stream := httpx.NewChatCompletionStream(&timeoutReadCloser{})
	defer stream.Close()

	_, err := stream.Next(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}
```

从 `internal/provider/httpx/httpx_test.go` imports 删除不再使用的：

```go
"time"
```

保留 `net/http` 和 `net/http/httptest`，因为同文件的正常 SSE 和 upstream error 测试仍使用它们。

- [ ] **Step 3: 删除 OpenAI provider 层重复的读取 timeout 测试**

从 `internal/provider/openai/openai_test.go` 删除整个函数：

```go
func TestStreamChatCompletionReadTimeoutIsDeadlineExceeded(t *testing.T) {
	headersFlushed := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		close(headersFlushed)
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	p, err := openai.New(server.URL+"/v1", "upstream-key", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()
	<-headersFlushed

	_, err = stream.Next(context.Background())
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}
```

不要删除 `time` import：同文件的建连 timeout 测试仍使用 `time.Sleep` 和 `time.Millisecond`。

- [ ] **Step 4: 格式化并运行聚焦测试**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "gofmt -w internal/provider/httpx/httpx_test.go internal/provider/openai/openai_test.go && go test -race ./internal/provider/httpx -run TestStreamReadTimeoutIsDeadlineExceeded -count=100 && go test -race ./internal/provider/openai -run TestStreamChatCompletionConnectTimeoutIsDeadlineExceeded -count=20"
```

Expected:

```text
ok  open-ai-gateway/internal/provider/httpx
ok  open-ai-gateway/internal/provider/openai
```

两个命令均 exit code `0`。确认新 `httpx` 测试不启动 server、不 sleep、也不使用 `http.Client.Timeout`。

- [ ] **Step 5: 新增 Task 153 文档**

创建 `tasks/153-stream-read-timeout-race-test-stability.md`：

```markdown
# Task 153 - Stream Read Timeout Race Test Stability

## 状态

Done.

## 背景

流式读取超时测试使用 `10ms` HTTP client timeout，并假定响应头阶段必定在 timeout 前完成。
在 `go test -race ./...` 的 instrumentation 和全量负载下，timeout 可能在
`StreamChatCompletion` 返回 stream 之前触发，导致测试偶发失败。

## 变更

- 在 `internal/provider/httpx` 使用立即返回 timeout error 的测试 reader，确定性验证
  stream 读取错误归一化为 `context.DeadlineExceeded`。
- 删除 OpenAI provider 中重复且依赖真实时间窗口的 stream read timeout 测试。
- 保留 OpenAI provider 建连 timeout 和现有 SSE 行为覆盖。
- 不修改生产代码。

## 验收

- `go test -race ./internal/provider/httpx -run TestStreamReadTimeoutIsDeadlineExceeded -count=100`
- `go test -race ./internal/provider/openai -run TestStreamChatCompletionConnectTimeoutIsDeadlineExceeded -count=20`
- `make verify`
```

- [ ] **Step 6: 运行完整 WSL 验证并检查变更范围**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "make verify"
git diff --check
git diff --stat
git status --short
```

Expected:

- `make verify` exit code `0`，包括 `go test ./...` 和 `go test -race ./...`。
- `git diff --check` 无输出。
- diff 只包含两个测试文件和 Task 153 文档；生产 Go 文件无变更。
- 若 WSL `gofmt -w` 仅造成 Windows 工作树换行符状态变化，应先确认 `git diff --ignore-space-at-eol` 无内容差异，再机械恢复换行符状态；不得把纯换行符变化纳入提交。

- [ ] **Step 7: 提交测试稳定性修复**

Run:

```powershell
git add internal/provider/httpx/httpx_test.go internal/provider/openai/openai_test.go tasks/153-stream-read-timeout-race-test-stability.md
git commit -m "test: stabilize stream read timeout race coverage"
git status --short --branch
```

Expected:

- commit 成功。
- 工作区干净。
- 当前分支相对 `origin/master` 包含设计提交、计划提交（若计划单独提交）和本任务实现提交。


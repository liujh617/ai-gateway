# Azure OpenAI Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an `azure-openai` provider type that supports Azure OpenAI chat completions, streaming chat completions, and embeddings through deployment endpoints.

**Architecture:** Keep handler, router, and compat boundaries unchanged. Extract provider HTTP response/SSE helpers from `internal/provider/openai` into a small shared `internal/provider/httpx` package, then implement Azure endpoint construction and `api-key` authentication in `internal/provider/azureopenai`.

**Tech Stack:** Go standard library only, `net/http`, `encoding/json`, existing `compat`, `provider`, `requestctx`, `upstreamurl`, and `version` packages.

---

## File Structure

- Create `internal/provider/httpx/httpx.go`: shared upstream HTTP helpers for JSON decoding, Content-Type checks, SSE stream parsing, timeout normalization, and upstream error mapping.
- Create `internal/provider/httpx/httpx_test.go`: focused regression tests for shared SSE and error behavior moved from OpenAI provider.
- Modify `internal/provider/openai/openai.go`: replace local helper implementations with `httpx` calls while keeping endpoint and header behavior unchanged.
- Modify `internal/provider/openai/openai_test.go`: keep OpenAI provider behavior tests passing; remove no tests unless duplicated by `httpx` tests and still covered elsewhere.
- Create `internal/provider/azureopenai/azureopenai.go`: Azure OpenAI provider implementation.
- Create `internal/provider/azureopenai/azureopenai_test.go`: Azure-specific request path, query, header, body, stream, error, and timeout tests.
- Modify `internal/config/config.go`: add `api_version` to provider config, validation, and check report.
- Modify `cmd/gateway/main.go`: construct `azure-openai` providers.
- Modify `cmd/gateway/main_test.go`: add provider construction coverage for Azure config where useful.
- Modify `schema/config.schema.json`: allow `azure-openai` and `api_version`.
- Add `config.azure-openai.example.json`: safe Azure config with non-secret example values.
- Modify `internal/config/examples_test.go`: include the new example in schema/config validation if needed by existing test discovery.
- Modify `README.md`, `openai-compatible-proxy-spec.md`, `architecture/overview.md`: document Azure provider contract.
- Add `tasks/152-azure-openai-provider.md`: task record for this feature.

---

### Task 1: Extract Shared Provider HTTP Helpers

**Files:**
- Create: `internal/provider/httpx/httpx.go`
- Create: `internal/provider/httpx/httpx_test.go`
- Modify: `internal/provider/openai/openai.go`

- [ ] **Step 1: Create failing package-level tests for shared SSE and error helpers**

Create `internal/provider/httpx/httpx_test.go` with tests that assert the shared stream parser and error mapper behave like the current OpenAI provider helpers:

```go
package httpx_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/httpx"
)

func TestStreamReadsSSEAndDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chunk\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	defer resp.Body.Close()
	if err := httpx.RequireEventStreamResponse(resp); err != nil {
		t.Fatalf("RequireEventStreamResponse: %v", err)
	}
	stream := httpx.NewChatCompletionStream(resp.Body)
	defer stream.Close()

	chunk, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if chunk.Choices[0].Delta.Content != "hi" {
		t.Fatalf("content = %q", chunk.Choices[0].Delta.Content)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("done err = %v, want EOF", err)
	}
}

func TestUpstreamErrorMapping(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream exploded","type":"server_error","code":"upstream_error"}}`)),
	}
	resp.Header.Set("Content-Type", "application/json")

	err := httpx.UpstreamError(resp)
	compatErr, ok := err.(*compat.Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if compatErr.Status != http.StatusBadGateway {
		t.Fatalf("status = %d", compatErr.Status)
	}
	if compatErr.Message != "upstream exploded" || compatErr.Type != "server_error" {
		t.Fatalf("mapped error = %+v", compatErr)
	}
	if compatErr.Code == nil || *compatErr.Code != "upstream_error" {
		t.Fatalf("code = %v", compatErr.Code)
	}
}

func TestTransportTimeoutIsDeadlineExceeded(t *testing.T) {
	err := httpx.TransportError(&timeoutError{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}

type timeoutError struct{}

func (*timeoutError) Error() string   { return "timeout" }
func (*timeoutError) Timeout() bool   { return true }
func (*timeoutError) Temporary() bool { return true }

func TestStreamReadTimeoutIsDeadlineExceeded(t *testing.T) {
	headersFlushed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		close(headersFlushed)
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 10 * time.Millisecond}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	defer resp.Body.Close()
	<-headersFlushed

	stream := httpx.NewChatCompletionStream(resp.Body)
	_, err = stream.Next(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}
```

- [ ] **Step 2: Run the new tests and verify they fail**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/provider/httpx -count=1"
```

Expected: FAIL because `internal/provider/httpx` does not exist.

- [ ] **Step 3: Implement `internal/provider/httpx/httpx.go`**

Create `internal/provider/httpx/httpx.go` by moving the provider-agnostic helpers from `internal/provider/openai/openai.go`. Export the functions used by provider packages:

```go
package httpx

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"

	"open-ai-gateway/internal/compat"
)

const MaxResponseBodyBytes = 10 << 20

type ChatCompletionStream struct {
	body          io.ReadCloser
	reader        *bufio.Reader
	seenFirstLine bool
	skipNextLF    bool
}

func NewChatCompletionStream(body io.ReadCloser) *ChatCompletionStream {
	return &ChatCompletionStream{
		body:   body,
		reader: bufio.NewReader(body),
	}
}

func (s *ChatCompletionStream) Next(ctx context.Context) (*compat.ChatCompletionChunk, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		payload, err := s.nextPayload()
		if err != nil {
			return nil, err
		}
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			return nil, io.EOF
		}
		var chunk compat.ChatCompletionChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return nil, err
		}
		return &chunk, nil
	}
}

func (s *ChatCompletionStream) Close() error {
	return s.body.Close()
}

func (s *ChatCompletionStream) nextPayload() (string, error) {
	var data []string
	eventBytes := 0
	for {
		line, err := s.readSSELine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if line == "" {
					if len(data) == 0 {
						return "", io.EOF
					}
				}
				return "", errors.New("upstream SSE event ended without blank line")
			}
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if !s.seenFirstLine {
			s.seenFirstLine = true
			line = strings.TrimPrefix(line, "\ufeff")
		}
		if line == "" {
			eventBytes = 0
			if len(data) == 0 {
				continue
			}
			return strings.Join(data, "\n"), nil
		}
		eventBytes += len(line) + 1
		if eventBytes > MaxResponseBodyBytes {
			return "", fmt.Errorf("upstream SSE event exceeds %d bytes", MaxResponseBodyBytes)
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		if strings.HasPrefix(value, " ") {
			value = strings.TrimPrefix(value, " ")
		}
		switch field {
		case "data":
			data = append(data, value)
		case "event", "id", "retry":
			continue
		default:
			continue
		}
	}
}

func (s *ChatCompletionStream) readSSELine() (string, error) {
	var line strings.Builder
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if line.Len() == 0 {
					return "", io.EOF
				}
				return line.String(), io.EOF
			}
			return "", TransportError(err)
		}
		if s.skipNextLF {
			s.skipNextLF = false
			if b == '\n' {
				continue
			}
		}
		if line.Len()+1 > MaxResponseBodyBytes+1 {
			return "", fmt.Errorf("upstream SSE line exceeds %d bytes", MaxResponseBodyBytes)
		}
		line.WriteByte(b)
		if b == '\n' {
			return line.String(), nil
		}
		if b == '\r' {
			s.skipNextLF = true
			return line.String(), nil
		}
	}
}

func DecodeLimited(r io.Reader, out any) error {
	limited := &io.LimitedReader{R: r, N: MaxResponseBodyBytes + 1}
	decoder := json.NewDecoder(limited)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if limited.N <= 0 {
		return fmt.Errorf("upstream response body exceeds %d bytes", MaxResponseBodyBytes)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("upstream response body must contain a single JSON value")
		}
		return err
	}
	if limited.N <= 0 {
		return fmt.Errorf("upstream response body exceeds %d bytes", MaxResponseBodyBytes)
	}
	return nil
}

func RequireJSONResponse(resp *http.Response) error {
	if !ResponseContentTypeIs(resp, "application/json") {
		return fmt.Errorf("upstream response Content-Type must be application/json")
	}
	return nil
}

func RequireEventStreamResponse(resp *http.Response) error {
	if !ResponseContentTypeIs(resp, "text/event-stream") {
		return fmt.Errorf("upstream response Content-Type must be text/event-stream")
	}
	return nil
}

func ResponseContentTypeIs(resp *http.Response, want string) bool {
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, want)
}

func TransportError(err error) error {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf("upstream request timeout: %w", context.DeadlineExceeded)
	}
	return err
}

func UpstreamError(resp *http.Response) error {
	var upstream compat.ErrorResponse
	if ResponseContentTypeIs(resp, "application/json") {
		var decoded compat.ErrorResponse
		if err := DecodeLimited(resp.Body, &decoded); err == nil {
			upstream = decoded
		}
	}
	message := http.StatusText(resp.StatusCode)
	if upstream.Error.Message != "" {
		message = upstream.Error.Message
	}
	errorType := upstream.Error.Type
	if errorType == "" {
		errorType = defaultErrorType(resp.StatusCode)
	}
	status := resp.StatusCode
	if resp.StatusCode >= 500 && resp.StatusCode != http.StatusGatewayTimeout {
		status = http.StatusBadGateway
	}
	return &compat.Error{
		Status:  status,
		Message: message,
		Type:    errorType,
		Param:   upstream.Error.Param,
		Code:    upstream.Error.Code,
	}
}

func defaultErrorType(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "authentication_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusBadRequest, http.StatusNotFound:
		return "invalid_request_error"
	default:
		return "server_error"
	}
}
```

- [ ] **Step 4: Update OpenAI provider to use `httpx`**

Modify `internal/provider/openai/openai.go`:

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/provider/httpx"
	"open-ai-gateway/internal/requestctx"
	"open-ai-gateway/internal/upstreamurl"
	"open-ai-gateway/internal/version"
)
```

Replace calls:

```go
return nil, httpx.TransportError(err)
return nil, httpx.UpstreamError(resp)
if err := httpx.RequireJSONResponse(resp); err != nil { ... }
if err := httpx.RequireEventStreamResponse(resp); err != nil { ... }
if err := httpx.DecodeLimited(resp.Body, &out); err != nil { ... }
return httpx.NewChatCompletionStream(resp.Body), nil
```

Remove the local `stream` type and helper functions that were moved to `httpx`.

- [ ] **Step 5: Run provider tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/provider/httpx ./internal/provider/openai -count=1"
```

Expected: PASS.

- [ ] **Step 6: Commit shared helper extraction**

Run:

```powershell
git add internal/provider/httpx internal/provider/openai/openai.go internal/provider/openai/openai_test.go
git commit -m "Extract provider HTTP helpers"
```

---

### Task 2: Add Azure OpenAI Provider

**Files:**
- Create: `internal/provider/azureopenai/azureopenai.go`
- Create: `internal/provider/azureopenai/azureopenai_test.go`

- [ ] **Step 1: Write failing Azure provider tests**

Create `internal/provider/azureopenai/azureopenai_test.go`:

```go
package azureopenai_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/azureopenai"
	"open-ai-gateway/internal/requestctx"
	"open-ai-gateway/internal/version"
)

func TestCreateChatCompletionForwardsAzureRequest(t *testing.T) {
	var got compat.ChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/chat-deployment/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if apiVersion := r.URL.Query().Get("api-version"); apiVersion != "2024-02-15-preview" {
			t.Fatalf("api-version = %q", apiVersion)
		}
		assertCommonHeaders(t, r, "application/json")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl_azure","object":"chat.completion","created":1,"model":"chat-deployment","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	ctx := requestctx.WithRequestID(context.Background(), "gateway-request-1")
	resp, err := p.CreateChatCompletion(ctx, compat.ChatCompletionRequest{
		Model: "chat-deployment",
		Messages: []compat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hello"`),
		}},
		Stream: true,
		Extra: map[string]json.RawMessage{
			"tool_choice": json.RawMessage(`"auto"`),
		},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	if got.Model != "chat-deployment" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.Stream {
		t.Fatal("non-stream request forwarded with stream=true")
	}
	if string(got.Extra["tool_choice"]) != `"auto"` {
		t.Fatalf("tool_choice = %s", got.Extra["tool_choice"])
	}
	if resp.ID != "chatcmpl_azure" {
		t.Fatalf("response id = %q", resp.ID)
	}
}

func TestStreamChatCompletionForwardsAzureRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/chat-deployment/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if apiVersion := r.URL.Query().Get("api-version"); apiVersion != "2024-02-15-preview" {
			t.Fatalf("api-version = %q", apiVersion)
		}
		assertCommonHeaders(t, r, "text/event-stream")
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chunk\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"chat-deployment\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()
	chunk, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if chunk.Choices[0].Delta.Content != "hi" {
		t.Fatalf("content = %q", chunk.Choices[0].Delta.Content)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("done err = %v, want EOF", err)
	}
}

func TestCreateEmbeddingForwardsAzureRequest(t *testing.T) {
	var got compat.EmbeddingRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/embedding-deployment/embeddings" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if apiVersion := r.URL.Query().Get("api-version"); apiVersion != "2024-02-15-preview" {
			t.Fatalf("api-version = %q", apiVersion)
		}
		assertCommonHeaders(t, r, "application/json")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"object":"list","model":"embedding-deployment","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"usage":{"prompt_tokens":1,"total_tokens":1}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	resp, err := p.CreateEmbedding(context.Background(), compat.EmbeddingRequest{
		Model: "embedding-deployment",
		Input: json.RawMessage(`"hello"`),
		Extra: map[string]json.RawMessage{
			"dimensions": json.RawMessage(`512`),
		},
	})
	if err != nil {
		t.Fatalf("CreateEmbedding: %v", err)
	}
	if got.Model != "embedding-deployment" {
		t.Fatalf("model = %q", got.Model)
	}
	if string(got.Extra["dimensions"]) != `512` {
		t.Fatalf("dimensions = %s", got.Extra["dimensions"])
	}
	if resp.Model != "embedding-deployment" {
		t.Fatalf("response model = %q", resp.Model)
	}
}

func TestNewRejectsMissingAPIVersion(t *testing.T) {
	_, err := azureopenai.New("https://example.openai.azure.com", "key", "", 0)
	if err == nil {
		t.Fatal("expected api_version error")
	}
	if !strings.Contains(err.Error(), "api_version") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewRejectsBaseURLWithQueryOrFragment(t *testing.T) {
	for _, baseURL := range []string{
		"https://example.openai.azure.com?tenant=one",
		"https://example.openai.azure.com#frag",
	} {
		t.Run(baseURL, func(t *testing.T) {
			_, err := azureopenai.New(baseURL, "key", "2024-02-15-preview", 0)
			if err == nil {
				t.Fatal("expected base_url error")
			}
		})
	}
}

func TestCreateChatCompletionMapsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"slow down","type":"rate_limit_error","code":"rate_limit_exceeded"}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL)
	_, err := p.CreateChatCompletion(context.Background(), chatRequest())
	if err == nil {
		t.Fatal("expected error")
	}
	compatErr, ok := err.(*compat.Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if compatErr.Status != http.StatusTooManyRequests || compatErr.Type != "rate_limit_error" {
		t.Fatalf("mapped error = %+v", compatErr)
	}
}

func TestCreateChatCompletionTransportTimeoutIsDeadlineExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	p, err := azureopenai.New(server.URL, "azure-key", "2024-02-15-preview", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	_, err = p.CreateChatCompletion(context.Background(), chatRequest())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}

func TestListModelsReturnsEmptyList(t *testing.T) {
	p, err := azureopenai.New("https://example.openai.azure.com", "azure-key", "2024-02-15-preview", 0)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("models = %+v", models)
	}
}

func assertCommonHeaders(t *testing.T, r *http.Request, accept string) {
	t.Helper()
	if got := r.Header.Get("Accept"); got != accept {
		t.Fatalf("Accept = %q", got)
	}
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := r.Header.Get("api-key"); got != "azure-key" {
		t.Fatalf("api-key = %q", got)
	}
	if got := r.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := r.Header.Get("User-Agent"); got != version.UserAgent() {
		t.Fatalf("User-Agent = %q", got)
	}
}

func newProvider(t *testing.T, baseURL string) *azureopenai.Provider {
	t.Helper()
	p, err := azureopenai.New(baseURL, "azure-key", "2024-02-15-preview", 0)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	return p
}

func chatRequest() compat.ChatCompletionRequest {
	return compat.ChatCompletionRequest{
		Model: "chat-deployment",
		Messages: []compat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hello"`),
		}},
	}
}
```

- [ ] **Step 2: Run Azure provider tests and verify they fail**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/provider/azureopenai -count=1"
```

Expected: FAIL because package `azureopenai` does not exist.

- [ ] **Step 3: Implement `internal/provider/azureopenai/azureopenai.go`**

Create `internal/provider/azureopenai/azureopenai.go`:

```go
package azureopenai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/provider/httpx"
	"open-ai-gateway/internal/requestctx"
	"open-ai-gateway/internal/upstreamurl"
	"open-ai-gateway/internal/version"
)

type Provider struct {
	baseURL    string
	apiKey     string
	apiVersion string
	client     *http.Client
}

func New(baseURL, apiKey, apiVersion string, timeout time.Duration) (*Provider, error) {
	baseURL, err := upstreamurl.NormalizeHTTPBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	apiVersion = strings.TrimSpace(apiVersion)
	if apiVersion == "" {
		return nil, fmt.Errorf("api_version is required")
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Provider{
		baseURL:    baseURL,
		apiKey:     apiKey,
		apiVersion: apiVersion,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (p *Provider) ListModels(ctx context.Context) ([]compat.Model, error) {
	return []compat.Model{}, nil
}

func (p *Provider) CreateChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(req.Model, "chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}
	var out compat.ChatCompletionResponse
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) StreamChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(req.Model, "chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireEventStreamResponse(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}
	return httpx.NewChatCompletionStream(resp.Body), nil
}

func (p *Provider) CreateEmbedding(ctx context.Context, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(req.Model, "embeddings"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}
	var out compat.EmbeddingResponse
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) endpoint(deployment, operation string) string {
	escapedDeployment := url.PathEscape(deployment)
	values := url.Values{}
	values.Set("api-version", p.apiVersion)
	return fmt.Sprintf("%s/openai/deployments/%s/%s?%s", p.baseURL, escapedDeployment, operation, values.Encode())
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())
	if requestID := requestctx.RequestID(req.Context()); requestID != "" {
		req.Header.Set(requestctx.RequestIDHeader, requestID)
	}
	if p.apiKey != "" {
		req.Header.Set("api-key", p.apiKey)
	}
}

func (p *Provider) setJSONHeaders(req *http.Request) {
	p.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
}
```

- [ ] **Step 4: Run Azure provider tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/provider/azureopenai -count=1"
```

Expected: PASS.

- [ ] **Step 5: Commit Azure provider**

Run:

```powershell
git add internal/provider/azureopenai
git commit -m "Add Azure OpenAI provider"
```

---

### Task 3: Wire Azure Provider Into Config and Gateway

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/gateway/main.go`
- Modify: `cmd/gateway/main_test.go`

- [ ] **Step 1: Add failing config validation tests**

Append tests to `internal/config/config_test.go`:

```go
func TestValidateAcceptsAzureOpenAIProvider(t *testing.T) {
	cfg := config.Default()
	cfg.Providers = map[string]config.ProviderConfig{
		"azure": {
			Type:       "azure-openai",
			BaseURL:    "https://example.openai.azure.com",
			APIKeyEnv:  "AZURE_OPENAI_API_KEY",
			APIVersion: "2024-02-15-preview",
		},
	}
	cfg.Models = map[string]config.ModelConfig{
		"gpt-4o-mini": {
			Provider:      "azure",
			UpstreamModel: "chat-deployment",
			Capabilities:  []string{"chat"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	report := cfg.CheckReport()
	if len(report.Providers) != 1 || report.Providers[0].APIVersion != "2024-02-15-preview" {
		t.Fatalf("providers = %+v", report.Providers)
	}
}

func TestValidateRejectsAzureOpenAIProviderWithoutAPIVersion(t *testing.T) {
	cfg := config.Default()
	cfg.Providers = map[string]config.ProviderConfig{
		"azure": {
			Type:      "azure-openai",
			BaseURL:   "https://example.openai.azure.com",
			APIKeyEnv: "AZURE_OPENAI_API_KEY",
		},
	}
	cfg.Models = map[string]config.ModelConfig{
		"gpt-4o-mini": {
			Provider: "azure",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected api_version error")
	}
	if !strings.Contains(err.Error(), "api_version") {
		t.Fatalf("error = %v", err)
	}
}
```

If `internal/config/config_test.go` already imports `strings`, reuse it. If not, add it.

- [ ] **Step 2: Run config tests and verify they fail**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/config -run 'TestValidateAcceptsAzureOpenAIProvider|TestValidateRejectsAzureOpenAIProviderWithoutAPIVersion' -count=1"
```

Expected: FAIL because `ProviderConfig.APIVersion` and `ProviderSummary.APIVersion` do not exist.

- [ ] **Step 3: Update config structs and validation**

Modify `internal/config/config.go`:

```go
type ProviderConfig struct {
	Type           string `json:"type"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	APIKeyEnv      string `json:"api_key_env"`
	APIVersion     string `json:"api_version"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}
```

```go
type ProviderSummary struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	BaseURL        string `json:"base_url"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	APIKeySet      bool   `json:"api_key_set"`
	APIKeyEnv      string `json:"api_key_env"`
	APIKeyEnvSet   bool   `json:"api_key_env_set"`
	APIVersion     string `json:"api_version,omitempty"`
}
```

Set summary field:

```go
summary := ProviderSummary{
	Name:           name,
	Type:           provider.Type,
	BaseURL:        provider.BaseURL,
	TimeoutSeconds: provider.TimeoutSeconds,
	APIKeySet:      provider.APIKey != "",
	APIKeyEnv:      provider.APIKeyEnv,
	APIVersion:     provider.APIVersion,
}
```

Extend validation:

```go
switch provider.Type {
case "fake":
case "openai-compatible":
	if _, err := upstreamurl.NormalizeHTTPBaseURL(provider.BaseURL); err != nil {
		return fmt.Errorf("provider %q %w", name, err)
	}
	if provider.APIKey == "" && provider.APIKeyEnv == "" {
		return fmt.Errorf("provider %q requires api_key or api_key_env", name)
	}
case "azure-openai":
	if _, err := upstreamurl.NormalizeHTTPBaseURL(provider.BaseURL); err != nil {
		return fmt.Errorf("provider %q %w", name, err)
	}
	if provider.APIKey == "" && provider.APIKeyEnv == "" {
		return fmt.Errorf("provider %q requires api_key or api_key_env", name)
	}
	if strings.TrimSpace(provider.APIVersion) == "" {
		return fmt.Errorf("provider %q api_version is required", name)
	}
	if provider.APIVersion != strings.TrimSpace(provider.APIVersion) {
		return fmt.Errorf("provider %q api_version must not contain leading or trailing whitespace", name)
	}
default:
	return fmt.Errorf("provider %q has unsupported type %q", name, provider.Type)
}
```

- [ ] **Step 4: Wire gateway provider construction**

Modify imports in `cmd/gateway/main.go`:

```go
"open-ai-gateway/internal/provider/azureopenai"
```

Add build branch:

```go
case "azure-openai":
	provider, err := azureopenai.New(providerConfig.BaseURL, providerConfig.ResolvedAPIKey(), providerConfig.APIVersion, providerConfig.Timeout())
	if err != nil {
		return nil, fmt.Errorf("provider %q: %w", name, err)
	}
	providers[name] = provider
```

- [ ] **Step 5: Add gateway buildRouter test**

Append to `cmd/gateway/main_test.go`:

```go
func TestBuildRouterAcceptsAzureOpenAIProvider(t *testing.T) {
	cfg := config.Default()
	cfg.Providers = map[string]config.ProviderConfig{
		"azure": {
			Type:       "azure-openai",
			BaseURL:    "https://example.openai.azure.com",
			APIKey:     "azure-key",
			APIVersion: "2024-02-15-preview",
		},
	}
	cfg.Models = map[string]config.ModelConfig{
		"gpt-4o-mini": {
			Provider:      "azure",
			UpstreamModel: "chat-deployment",
			Capabilities:  []string{"chat"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := buildRouter(cfg); err != nil {
		t.Fatalf("buildRouter: %v", err)
	}
}
```

Add imports to `cmd/gateway/main_test.go` if missing:

```go
"open-ai-gateway/internal/config"
```

- [ ] **Step 6: Run config and gateway tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/config ./cmd/gateway -count=1"
```

Expected: PASS.

- [ ] **Step 7: Commit config and gateway wiring**

Run:

```powershell
git add internal/config/config.go internal/config/config_test.go cmd/gateway/main.go cmd/gateway/main_test.go
git commit -m "Wire Azure OpenAI provider config"
```

---

### Task 4: Add Schema and Example Config

**Files:**
- Modify: `schema/config.schema.json`
- Add: `config.azure-openai.example.json`
- Modify: `internal/config/examples_test.go` if explicit examples are listed there

- [ ] **Step 1: Write the Azure example config**

Create `config.azure-openai.example.json`:

```json
{
  "addr": "127.0.0.1:8080",
  "api_key": "test-gateway-key",
  "request_timeout_seconds": 60,
  "stream_timeout_seconds": 600,
  "read_header_timeout_seconds": 10,
  "read_timeout_seconds": 0,
  "write_timeout_seconds": 0,
  "idle_timeout_seconds": 120,
  "shutdown_timeout_seconds": 10,
  "max_request_body_bytes": 1048576,
  "log": {
    "format": "json",
    "level": "info"
  },
  "providers": {
    "azure": {
      "type": "azure-openai",
      "base_url": "https://your-resource.openai.azure.com",
      "api_key_env": "AZURE_OPENAI_API_KEY",
      "api_version": "2024-02-15-preview",
      "timeout_seconds": 60
    }
  },
  "models": {
    "gpt-4o-mini": {
      "provider": "azure",
      "upstream_model": "your-chat-deployment",
      "capabilities": ["chat"]
    },
    "text-embedding-3-small": {
      "provider": "azure",
      "upstream_model": "your-embedding-deployment",
      "capabilities": ["embeddings"]
    }
  }
}
```

- [ ] **Step 2: Run example validation and verify it fails before schema update**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "make check-config-examples"
```

Expected: FAIL if examples are discovered automatically and schema still rejects `azure-openai` or `api_version`.

- [ ] **Step 3: Update JSON Schema**

Modify `schema/config.schema.json` provider definition:

```json
"type": {
  "type": "string",
  "enum": ["fake", "openai-compatible", "azure-openai"]
},
```

Add provider property:

```json
"api_version": {
  "type": "string",
  "description": "Azure OpenAI api-version query value. Required for azure-openai providers."
},
```

Keep `additionalProperties: false`.

- [ ] **Step 4: Include example in tests if needed**

Open `internal/config/examples_test.go`. If it has a hard-coded list of example config files, add:

```go
"config.azure-openai.example.json",
```

If it discovers `config.*.example.json` automatically, do not modify the test.

- [ ] **Step 5: Run schema and example tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/config -count=1 && make check-config-examples"
```

Expected: PASS.

- [ ] **Step 6: Commit schema and example config**

Run:

```powershell
git add schema/config.schema.json config.azure-openai.example.json internal/config/examples_test.go
git commit -m "Add Azure OpenAI config example"
```

---

### Task 5: Update Docs and Task Record

**Files:**
- Modify: `README.md`
- Modify: `openai-compatible-proxy-spec.md`
- Modify: `architecture/overview.md`
- Add: `tasks/152-azure-openai-provider.md`

- [ ] **Step 1: Update README provider config section**

In `README.md`, add bullets near existing provider config bullets:

```markdown
- `providers.<name>.type`: 当前支持 `fake`、`openai-compatible` 和 `azure-openai`。
- `providers.<name>.api_version`: Azure OpenAI `api-version` query 值；仅 `azure-openai` 必填。
```

Add an Azure example sentence near real upstream docs:

```markdown
Azure OpenAI 示例见 [config.azure-openai.example.json](config.azure-openai.example.json)。
```

- [ ] **Step 2: Update proxy spec**

In `openai-compatible-proxy-spec.md`, update provider type and model mapping sections with:

```markdown
`azure-openai` provider 使用 Azure deployment endpoint：

```text
POST <base_url>/openai/deployments/<deployment>/chat/completions?api-version=<api_version>
POST <base_url>/openai/deployments/<deployment>/embeddings?api-version=<api_version>
```

Azure provider 使用 `api-key` header 发送上游 API key。`models.<external>.upstream_model` 对 Azure 表示 deployment name。
```
```

- [ ] **Step 3: Update architecture overview**

In `architecture/overview.md`, update current provider implementation list:

```markdown
- `internal/provider/azureopenai`: Azure OpenAI provider，转发 Azure deployment 形式的 chat completions、streaming chat completions 和 embeddings。
```

- [ ] **Step 4: Add task record**

Create `tasks/152-azure-openai-provider.md`:

```markdown
# Task 152 - Azure OpenAI Provider

## 背景

0.1.0 已支持标准 OpenAI-compatible upstream provider。Azure OpenAI 使用 deployment endpoint 和 `api-version` query，与标准 `/v1/chat/completions` 路径不同，需要独立 provider adapter。

## 变更

- 新增 `azure-openai` provider type。
- 新增 Azure OpenAI deployment endpoint 构造。
- 使用 `api-key` header 发送上游 API key。
- 支持 chat completions、streaming chat completions 和 embeddings。
- 新增 `providers.<name>.api_version` 配置和 check-config summary。
- 同步 schema、示例配置、README、兼容契约和架构文档。

## 验证

- `go test ./internal/provider/httpx ./internal/provider/openai ./internal/provider/azureopenai -count=1`
- `go test ./internal/config ./cmd/gateway -count=1`
- `make verify`
- `make check-config-examples`
```

- [ ] **Step 5: Commit docs**

Run:

```powershell
git add README.md openai-compatible-proxy-spec.md architecture/overview.md tasks/152-azure-openai-provider.md
git commit -m "Document Azure OpenAI provider"
```

---

### Task 6: Final Verification

**Files:**
- No code changes expected.

- [ ] **Step 1: Run focused provider and config tests**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./internal/provider/httpx ./internal/provider/openai ./internal/provider/azureopenai ./internal/config ./cmd/gateway -count=1"
```

Expected: PASS.

- [ ] **Step 2: Run full project verification**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "make verify"
```

Expected: PASS.

- [ ] **Step 3: Run config examples check**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "make check-config-examples"
```

Expected: PASS and output includes `config-examples-ok`.

- [ ] **Step 4: Inspect git status**

Run:

```powershell
git status --short --branch
```

Expected: clean working tree, branch ahead by the implementation commits.

- [ ] **Step 5: Report completion with evidence**

Final report must include:

- Focused test command result.
- `make verify` result.
- `make check-config-examples` result.
- Commit list created during implementation.

---

## Self-Review

- Spec coverage: Azure provider type, deployment endpoints, `api-version`, `api-key`, chat, stream, embeddings, config validation, check-config summary, schema, example config, docs, and task record are covered.
- Scope check: The plan excludes generic path/header/query templates, Azure AD token auth, and additional OpenAI APIs, matching the spec.
- Type consistency: The plan uses `APIVersion` in Go structs and `api_version` in JSON. Provider package name is consistently `azureopenai`; provider type string is consistently `azure-openai`.

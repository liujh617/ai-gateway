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
		if requestID := r.Header.Get(requestctx.RequestIDHeader); requestID != "gateway-request-1" {
			t.Fatalf("request id = %q", requestID)
		}
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

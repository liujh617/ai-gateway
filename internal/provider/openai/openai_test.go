package openai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/openai"
)

func TestCreateChatCompletionForwardsRequest(t *testing.T) {
	var got compat.ChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer upstream-key" {
			t.Fatalf("authorization = %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl_upstream","object":"chat.completion","created":1,"model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	resp, err := p.CreateChatCompletion(context.Background(), compat.ChatCompletionRequest{
		Model: "upstream-model",
		Messages: []compat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hello"`),
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	if got.Model != "upstream-model" {
		t.Fatalf("forwarded model = %q", got.Model)
	}
	if got.Stream {
		t.Fatal("non-stream request forwarded with stream=true")
	}
	if resp.ID != "chatcmpl_upstream" {
		t.Fatalf("response id = %q", resp.ID)
	}
}

func TestStreamChatCompletionReadsSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chatcmpl_upstream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hel\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: {\"id\":\"chatcmpl_upstream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), compat.ChatCompletionRequest{
		Model: "upstream-model",
		Messages: []compat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hello"`),
		}},
	})
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	first, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("first chunk: %v", err)
	}
	if first.Choices[0].Delta.Content != "hel" {
		t.Fatalf("first content = %q", first.Choices[0].Delta.Content)
	}
	second, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("second chunk: %v", err)
	}
	if second.Choices[0].Delta.Content != "lo" {
		t.Fatalf("second content = %q", second.Choices[0].Delta.Content)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("end error = %v, want EOF", err)
	}
}

func TestUpstreamErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"slow down","type":"rate_limit_error","param":null,"code":null}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateChatCompletion(context.Background(), compat.ChatCompletionRequest{
		Model: "upstream-model",
		Messages: []compat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hello"`),
		}},
	})
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

func newProvider(t *testing.T, baseURL string) *openai.Provider {
	t.Helper()
	p, err := openai.New(baseURL, "upstream-key", 0)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	return p
}

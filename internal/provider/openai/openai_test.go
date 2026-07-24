package openai_test

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
	"open-ai-gateway/internal/provider/openai"
	"open-ai-gateway/internal/requestctx"
	"open-ai-gateway/internal/version"
)

func TestCreateChatCompletionForwardsRequest(t *testing.T) {
	var got compat.ChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Fatalf("accept = %q", accept)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("content-type = %q", contentType)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer upstream-key" {
			t.Fatalf("authorization = %q", auth)
		}
		if requestID := r.Header.Get(requestctx.RequestIDHeader); requestID != "gateway-request-1" {
			t.Fatalf("request id = %q", requestID)
		}
		if userAgent := r.Header.Get("User-Agent"); userAgent != version.UserAgent() {
			t.Fatalf("user-agent = %q", userAgent)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl_upstream","object":"chat.completion","created":1,"model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	ctx := requestctx.WithRequestID(context.Background(), "gateway-request-1")
	resp, err := p.CreateChatCompletion(ctx, compat.ChatCompletionRequest{
		Model: "upstream-model",
		Messages: []compat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hello"`),
		}},
		Stream: true,
		Extra: map[string]json.RawMessage{
			"model":           json.RawMessage(`"wrong-model"`),
			"stream":          json.RawMessage(`true`),
			"tool_choice":     json.RawMessage(`"auto"`),
			"response_format": json.RawMessage(`{"type":"json_object"}`),
		},
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
	if string(got.Extra["tool_choice"]) != `"auto"` {
		t.Fatalf("forwarded tool_choice = %s", got.Extra["tool_choice"])
	}
	if string(got.Extra["response_format"]) != `{"type":"json_object"}` {
		t.Fatalf("forwarded response_format = %s", got.Extra["response_format"])
	}
	if resp.ID != "chatcmpl_upstream" {
		t.Fatalf("response id = %q", resp.ID)
	}
}

func TestNewRejectsNonHTTPBaseURL(t *testing.T) {
	_, err := openai.New("ftp://api.example.com/v1", "upstream-key", 0)
	if err == nil {
		t.Fatal("expected base_url scheme error")
	}
	if !strings.Contains(err.Error(), "http or https") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewRejectsBaseURLWithQueryOrFragment(t *testing.T) {
	for _, baseURL := range []string{
		"https://api.example.com/v1?tenant=one",
		"https://api.example.com/v1#models",
	} {
		t.Run(baseURL, func(t *testing.T) {
			_, err := openai.New(baseURL, "upstream-key", 0)
			if err == nil {
				t.Fatal("expected base_url query or fragment error")
			}
			if !strings.Contains(err.Error(), "query or fragment") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestCreateChatCompletionTransportTimeoutIsDeadlineExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	p, err := openai.New(server.URL+"/v1", "upstream-key", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	_, err = p.CreateChatCompletion(context.Background(), chatRequest())
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}

func TestCreateEmbeddingTransportTimeoutIsDeadlineExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	p, err := openai.New(server.URL+"/v1", "upstream-key", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	_, err = p.CreateEmbedding(context.Background(), compat.EmbeddingRequest{
		Model: "upstream-embedding-model",
		Input: json.RawMessage(`"hello"`),
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}

func TestStreamChatCompletionConnectTimeoutIsDeadlineExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	p, err := openai.New(server.URL+"/v1", "upstream-key", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	_, err = p.StreamChatCompletion(context.Background(), chatRequest())
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}

func TestCreateEmbeddingForwardsRequest(t *testing.T) {
	var got compat.EmbeddingRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Fatalf("accept = %q", accept)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("content-type = %q", contentType)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer upstream-key" {
			t.Fatalf("authorization = %q", auth)
		}
		if requestID := r.Header.Get(requestctx.RequestIDHeader); requestID != "gateway-request-2" {
			t.Fatalf("request id = %q", requestID)
		}
		if userAgent := r.Header.Get("User-Agent"); userAgent != version.UserAgent() {
			t.Fatalf("user-agent = %q", userAgent)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"object":"list","model":"upstream-embedding-model","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":1,"total_tokens":1}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	ctx := requestctx.WithRequestID(context.Background(), "gateway-request-2")
	resp, err := p.CreateEmbedding(ctx, compat.EmbeddingRequest{
		Model: "upstream-embedding-model",
		Input: json.RawMessage(`"hello"`),
		Extra: map[string]json.RawMessage{
			"model":      json.RawMessage(`"wrong-model"`),
			"dimensions": json.RawMessage(`512`),
		},
	})
	if err != nil {
		t.Fatalf("CreateEmbedding: %v", err)
	}
	if got.Model != "upstream-embedding-model" {
		t.Fatalf("forwarded model = %q", got.Model)
	}
	if string(got.Input) != `"hello"` {
		t.Fatalf("forwarded input = %s", got.Input)
	}
	if string(got.Extra["dimensions"]) != `512` {
		t.Fatalf("forwarded dimensions = %s", got.Extra["dimensions"])
	}
	if resp.Object != "list" || len(resp.Data) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestStreamChatCompletionReadsSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if accept := r.Header.Get("Accept"); accept != "text/event-stream" {
			t.Fatalf("accept = %q", accept)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("content-type = %q", contentType)
		}
		if requestID := r.Header.Get(requestctx.RequestIDHeader); requestID != "gateway-request-3" {
			t.Fatalf("request id = %q", requestID)
		}
		if userAgent := r.Header.Get("User-Agent"); userAgent != version.UserAgent() {
			t.Fatalf("user-agent = %q", userAgent)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chatcmpl_upstream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hel\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: {\"id\":\"chatcmpl_upstream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	ctx := requestctx.WithRequestID(context.Background(), "gateway-request-3")
	stream, err := p.StreamChatCompletion(ctx, compat.ChatCompletionRequest{
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
	if second.Usage != nil {
		t.Fatalf("second usage = %+v", second.Usage)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("end error = %v, want EOF", err)
	}
}

func TestStreamChatCompletionReadsUsageChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chatcmpl_upstream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":5,\"total_tokens\":8}}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	chunk, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("usage chunk: %v", err)
	}
	if chunk.Usage == nil {
		t.Fatal("usage was not parsed")
	}
	if chunk.Usage.PromptTokens != 3 || chunk.Usage.CompletionTokens != 5 || chunk.Usage.TotalTokens != 8 {
		t.Fatalf("usage = %+v", chunk.Usage)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("end error = %v, want EOF", err)
	}
}

func TestStreamChatCompletionReadsMultilineSSEAndIgnoresMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, ": keepalive\n")
		io.WriteString(w, "event: completion\n")
		io.WriteString(w, "id: 1\n")
		io.WriteString(w, "retry: 1000\n")
		io.WriteString(w, "data: {\"id\":\"chatcmpl_upstream\",\"object\":\"chat.completion.chunk\",\"created\":1,\n")
		io.WriteString(w, "data: \"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "event: done\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	chunk, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("chunk: %v", err)
	}
	if chunk.Choices[0].Delta.Content != "hello" {
		t.Fatalf("content = %q", chunk.Choices[0].Delta.Content)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("end error = %v, want EOF", err)
	}
}

func TestStreamChatCompletionHandlesFieldWithoutColonAsEmptyValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("end error = %v, want EOF", err)
	}
}

func TestStreamChatCompletionIgnoresLeadingBOM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "\ufeffdata: {\"id\":\"chatcmpl_upstream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	chunk, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("chunk: %v", err)
	}
	if chunk.Choices[0].Delta.Content != "hello" {
		t.Fatalf("content = %q", chunk.Choices[0].Delta.Content)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("end error = %v, want EOF", err)
	}
}

func TestStreamChatCompletionAcceptsCRLineEndings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chatcmpl_upstream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\r\r")
		io.WriteString(w, "data: [DONE]\r\r")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	chunk, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("chunk: %v", err)
	}
	if chunk.Choices[0].Delta.Content != "hello" {
		t.Fatalf("content = %q", chunk.Choices[0].Delta.Content)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("end error = %v, want EOF", err)
	}
}

func TestStreamChatCompletionDoesNotWaitAfterCRLineEnding(t *testing.T) {
	firstEventWritten := make(chan struct{})
	releaseDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chatcmpl_upstream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\r\r")
		flusher.Flush()
		close(firstEventWritten)
		<-releaseDone
		io.WriteString(w, "data: [DONE]\r\r")
	}))
	defer server.Close()
	defer close(releaseDone)

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	<-firstEventWritten
	result := make(chan struct {
		chunk *compat.ChatCompletionChunk
		err   error
	}, 1)
	go func() {
		chunk, err := stream.Next(context.Background())
		result <- struct {
			chunk *compat.ChatCompletionChunk
			err   error
		}{chunk: chunk, err: err}
	}()

	select {
	case got := <-result:
		if got.err != nil {
			t.Fatalf("chunk: %v", got.err)
		}
		if got.chunk.Choices[0].Delta.Content != "hello" {
			t.Fatalf("content = %q", got.chunk.Choices[0].Delta.Content)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for CR-terminated event")
	}
}

func TestStreamChatCompletionAcceptsEventStreamWithCharset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("end error = %v, want EOF", err)
	}
}

func TestStreamChatCompletionRejectsNonEventStreamContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"error":"not a stream"}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err == nil {
		t.Fatal("expected Content-Type error")
	}
	if !strings.Contains(err.Error(), "text/event-stream") {
		t.Fatalf("error = %v", err)
	}
}

func TestStreamChatCompletionMalformedChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {not-json}\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	_, err = stream.Next(context.Background())
	if err == nil {
		t.Fatal("expected malformed chunk error")
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("error = %v", err)
	}
}

func TestStreamChatCompletionRejectsOversizedSSEEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: ")
		io.WriteString(w, strings.Repeat("x", 11<<20))
		io.WriteString(w, "\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	_, err = stream.Next(context.Background())
	if err == nil {
		t.Fatal("expected oversized SSE event error")
	}
	if !strings.Contains(err.Error(), "SSE") {
		t.Fatalf("error = %v", err)
	}
}

func TestStreamChatCompletionRejectsOversizedSSELineBeforeEventEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, ": ")
		io.WriteString(w, strings.Repeat("x", 11<<20))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	_, err = stream.Next(context.Background())
	if err == nil {
		t.Fatal("expected oversized SSE line error")
	}
	if !strings.Contains(err.Error(), "SSE line") {
		t.Fatalf("error = %v", err)
	}
}

func TestStreamChatCompletionRejectsUnterminatedSSEEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chatcmpl_upstream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	defer stream.Close()

	_, err = stream.Next(context.Background())
	if err == nil {
		t.Fatal("expected unterminated SSE event error")
	}
	if !strings.Contains(err.Error(), "blank line") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpstreamErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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

func TestUpstreamErrorMappingPreservesDetails(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantStatus int
		wantType   string
		wantCode   string
		wantParam  string
	}{
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			body:       `{"error":{"message":"bad key","type":"authentication_error","param":null,"code":"invalid_api_key"}}`,
			wantStatus: http.StatusUnauthorized,
			wantType:   "authentication_error",
			wantCode:   "invalid_api_key",
		},
		{
			name:       "forbidden",
			status:     http.StatusForbidden,
			body:       `{"error":{"message":"forbidden","type":"authentication_error","param":null,"code":"forbidden"}}`,
			wantStatus: http.StatusForbidden,
			wantType:   "authentication_error",
			wantCode:   "forbidden",
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			body:       `{"error":{"message":"model missing","type":"invalid_request_error","param":"model","code":"model_not_found"}}`,
			wantStatus: http.StatusNotFound,
			wantType:   "invalid_request_error",
			wantCode:   "model_not_found",
			wantParam:  "model",
		},
		{
			name:       "rate limit",
			status:     http.StatusTooManyRequests,
			body:       `{"error":{"message":"slow down","type":"rate_limit_error","param":null,"code":"rate_limit_exceeded"}}`,
			wantStatus: http.StatusTooManyRequests,
			wantType:   "rate_limit_error",
			wantCode:   "rate_limit_exceeded",
		},
		{
			name:       "server error",
			status:     http.StatusInternalServerError,
			body:       `{"error":{"message":"upstream exploded","type":"server_error","param":null,"code":"upstream_error"}}`,
			wantStatus: http.StatusBadGateway,
			wantType:   "server_error",
			wantCode:   "upstream_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				io.WriteString(w, tt.body)
			}))
			defer server.Close()

			p := newProvider(t, server.URL+"/v1")
			_, err := p.CreateChatCompletion(context.Background(), chatRequest())
			if err == nil {
				t.Fatal("expected error")
			}
			compatErr, ok := err.(*compat.Error)
			if !ok {
				t.Fatalf("error type = %T", err)
			}
			if compatErr.Status != tt.wantStatus || compatErr.Type != tt.wantType {
				t.Fatalf("mapped error = %+v", compatErr)
			}
			if tt.wantCode != "" && (compatErr.Code == nil || *compatErr.Code != tt.wantCode) {
				t.Fatalf("code = %v, want %q", compatErr.Code, tt.wantCode)
			}
			if tt.wantParam != "" && (compatErr.Param == nil || *compatErr.Param != tt.wantParam) {
				t.Fatalf("param = %v, want %q", compatErr.Param, tt.wantParam)
			}
		})
	}
}

func TestUpstreamErrorMappingIgnoresOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error":{"message":"do not trust","type":"rate_limit_error"}}`)
		io.WriteString(w, strings.Repeat(" ", 11<<20))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateChatCompletion(context.Background(), chatRequest())
	if err == nil {
		t.Fatal("expected error")
	}
	compatErr, ok := err.(*compat.Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if compatErr.Status != http.StatusBadGateway || compatErr.Type != "server_error" {
		t.Fatalf("mapped error = %+v", compatErr)
	}
	if compatErr.Message != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("message = %q", compatErr.Message)
	}
}

func TestUpstreamErrorMappingIgnoresTrailingJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"slow down","type":"server_error","code":"upstream_code"}}{}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
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
	if compatErr.Message != http.StatusText(http.StatusTooManyRequests) {
		t.Fatalf("message = %q", compatErr.Message)
	}
	if compatErr.Code != nil {
		t.Fatalf("code = %v", compatErr.Code)
	}
}

func TestUpstreamErrorMappingIgnoresNonJSONContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error":{"message":"do not trust","type":"rate_limit_error","code":"upstream_code"}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateChatCompletion(context.Background(), chatRequest())
	if err == nil {
		t.Fatal("expected error")
	}
	compatErr, ok := err.(*compat.Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if compatErr.Status != http.StatusBadGateway || compatErr.Type != "server_error" {
		t.Fatalf("mapped error = %+v", compatErr)
	}
	if compatErr.Message != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("message = %q", compatErr.Message)
	}
	if compatErr.Code != nil {
		t.Fatalf("code = %v", compatErr.Code)
	}
}

func TestCreateChatCompletionRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, strings.Repeat(" ", 11<<20))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateChatCompletion(context.Background(), chatRequest())
	if err == nil {
		t.Fatal("expected oversized or invalid response error")
	}
}

func TestListModelsRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, strings.Repeat(" ", 11<<20))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected oversized or invalid response error")
	}
}

func TestCreateEmbeddingRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, strings.Repeat(" ", 11<<20))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateEmbedding(context.Background(), compat.EmbeddingRequest{
		Model: "upstream-embedding-model",
		Input: json.RawMessage(`"hello"`),
	})
	if err == nil {
		t.Fatal("expected oversized or invalid response error")
	}
}

func TestCreateChatCompletionAcceptsJSONResponseWithCharset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		io.WriteString(w, `{"id":"chatcmpl_upstream","object":"chat.completion","created":1,"model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	resp, err := p.CreateChatCompletion(context.Background(), chatRequest())
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	if resp.ID != "chatcmpl_upstream" {
		t.Fatalf("response id = %q", resp.ID)
	}
}

func TestListModelsAcceptsJSONResponseWithCharset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		io.WriteString(w, `{"object":"list","data":[{"id":"upstream-model","object":"model","created":0,"owned_by":"upstream"}]}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0].ID != "upstream-model" {
		t.Fatalf("models = %+v", models)
	}
}

func TestCreateEmbeddingAcceptsJSONResponseWithCharset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		io.WriteString(w, `{"object":"list","model":"upstream-embedding-model","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":1,"total_tokens":1}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	resp, err := p.CreateEmbedding(context.Background(), compat.EmbeddingRequest{
		Model: "upstream-embedding-model",
		Input: json.RawMessage(`"hello"`),
	})
	if err != nil {
		t.Fatalf("CreateEmbedding: %v", err)
	}
	if resp.Model != "upstream-embedding-model" {
		t.Fatalf("response model = %q", resp.Model)
	}
}

func TestCreateChatCompletionRejectsNonJSONContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, `{"id":"chatcmpl_upstream","object":"chat.completion","created":1,"model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateChatCompletion(context.Background(), chatRequest())
	if err == nil {
		t.Fatal("expected Content-Type error")
	}
	if !strings.Contains(err.Error(), "Content-Type") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateEmbeddingRejectsNonJSONContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, `{"object":"list","model":"upstream-embedding-model","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":1,"total_tokens":1}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateEmbedding(context.Background(), compat.EmbeddingRequest{
		Model: "upstream-embedding-model",
		Input: json.RawMessage(`"hello"`),
	})
	if err == nil {
		t.Fatal("expected Content-Type error")
	}
	if !strings.Contains(err.Error(), "Content-Type") {
		t.Fatalf("error = %v", err)
	}
}

func TestListModelsForwardsHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Fatalf("accept = %q", accept)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "" {
			t.Fatalf("content-type = %q", contentType)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer upstream-key" {
			t.Fatalf("authorization = %q", auth)
		}
		if requestID := r.Header.Get(requestctx.RequestIDHeader); requestID != "gateway-request-models" {
			t.Fatalf("request id = %q", requestID)
		}
		if userAgent := r.Header.Get("User-Agent"); userAgent != version.UserAgent() {
			t.Fatalf("user-agent = %q", userAgent)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"object":"list","data":[{"id":"upstream-model","object":"model","created":0,"owned_by":"upstream"}]}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	ctx := requestctx.WithRequestID(context.Background(), "gateway-request-models")
	models, err := p.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0].ID != "upstream-model" {
		t.Fatalf("models = %+v", models)
	}
}

func TestProviderOmitsAuthorizationWithoutAPIKey(t *testing.T) {
	tests := []struct {
		name string
		path string
		call func(t *testing.T, p *openai.Provider)
	}{
		{
			name: "list models",
			path: "/v1/models",
			call: func(t *testing.T, p *openai.Provider) {
				t.Helper()
				if _, err := p.ListModels(context.Background()); err != nil {
					t.Fatalf("ListModels: %v", err)
				}
			},
		},
		{
			name: "chat completions",
			path: "/v1/chat/completions",
			call: func(t *testing.T, p *openai.Provider) {
				t.Helper()
				if _, err := p.CreateChatCompletion(context.Background(), chatRequest()); err != nil {
					t.Fatalf("CreateChatCompletion: %v", err)
				}
			},
		},
		{
			name: "stream chat completions",
			path: "/v1/chat/completions",
			call: func(t *testing.T, p *openai.Provider) {
				t.Helper()
				stream, err := p.StreamChatCompletion(context.Background(), chatRequest())
				if err != nil {
					t.Fatalf("StreamChatCompletion: %v", err)
				}
				if err := stream.Close(); err != nil {
					t.Fatalf("close stream: %v", err)
				}
			},
		},
		{
			name: "embeddings",
			path: "/v1/embeddings",
			call: func(t *testing.T, p *openai.Provider) {
				t.Helper()
				_, err := p.CreateEmbedding(context.Background(), compat.EmbeddingRequest{
					Model: "upstream-embedding-model",
					Input: json.RawMessage(`"hello"`),
				})
				if err != nil {
					t.Fatalf("CreateEmbedding: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.path {
					t.Fatalf("path = %s", r.URL.Path)
				}
				if auth := r.Header.Get("Authorization"); auth != "" {
					t.Fatalf("authorization = %q", auth)
				}
				switch tt.name {
				case "stream chat completions":
					w.Header().Set("Content-Type", "text/event-stream")
					io.WriteString(w, "data: [DONE]\n\n")
				case "chat completions":
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, `{"id":"chatcmpl_upstream","object":"chat.completion","created":1,"model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`)
				case "embeddings":
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, `{"object":"list","model":"upstream-embedding-model","data":[{"object":"embedding","index":0,"embedding":[0.1]}]}`)
				default:
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, `{"object":"list","data":[]}`)
				}
			}))
			defer server.Close()

			p, err := openai.New(server.URL+"/v1", "", 0)
			if err != nil {
				t.Fatalf("new provider: %v", err)
			}
			tt.call(t, p)
		})
	}
}

func TestListModelsTransportTimeoutIsDeadlineExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	p, err := openai.New(server.URL+"/v1", "upstream-key", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	_, err = p.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}

func TestListModelsRejectsNonJSONContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, `{"object":"list","data":[{"id":"upstream-model","object":"model","created":0,"owned_by":"upstream"}]}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected Content-Type error")
	}
	if !strings.Contains(err.Error(), "Content-Type") {
		t.Fatalf("error = %v", err)
	}
}

func TestListModelsRejectsTrailingJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"object":"list","data":[{"id":"upstream-model","object":"model","created":0,"owned_by":"upstream"}]}{}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected trailing JSON response error")
	}
	if !strings.Contains(err.Error(), "single JSON value") {
		t.Fatalf("error = %v", err)
	}
}

func TestListModelsMapsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":{"message":"model endpoint missing","type":"invalid_request_error","code":"not_found"}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	compatErr, ok := err.(*compat.Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if compatErr.Status != http.StatusNotFound || compatErr.Type != "invalid_request_error" || compatErr.Message != "model endpoint missing" {
		t.Fatalf("mapped error = %+v", compatErr)
	}
	if compatErr.Code == nil || *compatErr.Code != "not_found" {
		t.Fatalf("code = %v", compatErr.Code)
	}
}

func TestCreateChatCompletionRejectsTrailingJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl_upstream","object":"chat.completion","created":1,"model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}{}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateChatCompletion(context.Background(), chatRequest())
	if err == nil {
		t.Fatal("expected trailing JSON response error")
	}
	if !strings.Contains(err.Error(), "single JSON value") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateEmbeddingRejectsTrailingJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"object":"list","model":"upstream-embedding-model","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":1,"total_tokens":1}}{}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateEmbedding(context.Background(), compat.EmbeddingRequest{
		Model: "upstream-embedding-model",
		Input: json.RawMessage(`"hello"`),
	})
	if err == nil {
		t.Fatal("expected trailing JSON response error")
	}
	if !strings.Contains(err.Error(), "single JSON value") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateEmbeddingMapsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":{"message":"model missing","type":"invalid_request_error","param":"model","code":"model_not_found"}}`)
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateEmbedding(context.Background(), compat.EmbeddingRequest{
		Model: "upstream-embedding-model",
		Input: json.RawMessage(`"hello"`),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	compatErr, ok := err.(*compat.Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if compatErr.Status != http.StatusNotFound || compatErr.Type != "invalid_request_error" {
		t.Fatalf("mapped error = %+v", compatErr)
	}
}

func intPtr(v int) *int { return &v }

func newProvider(t *testing.T, baseURL string) *openai.Provider {
	t.Helper()
	p, err := openai.New(baseURL, "upstream-key", 0)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	return p
}

func chatRequest() compat.ChatCompletionRequest {
	return compat.ChatCompletionRequest{
		Model: "upstream-model",
		Messages: []compat.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(`"hello"`),
		}},
	}
}

func TestCreateSpeechForwardsRequest(t *testing.T) {
	var got compat.SpeechRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type = %q", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("fake-mp3-data"))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	resp, err := p.CreateSpeech(context.Background(), compat.SpeechRequest{
		Model: "tts-1",
		Input: "hello world",
		Voice: "alloy",
	})
	if err != nil {
		t.Fatalf("CreateSpeech: %v", err)
	}
	if string(resp.Data) != "fake-mp3-data" {
		t.Fatalf("data = %q", resp.Data)
	}
	if resp.ContentType != "audio/mpeg" {
		t.Fatalf("content-type = %q", resp.ContentType)
	}
	if got.Model != "tts-1" || got.Input != "hello world" || got.Voice != "alloy" {
		t.Fatalf("request = %#v", got)
	}
}

func TestCreateSpeechUsesUpstreamContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/ogg")
		w.Write([]byte("audio-data"))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	resp, err := p.CreateSpeech(context.Background(), compat.SpeechRequest{Model: "tts-1", Input: "hi", Voice: "alloy"})
	if err != nil {
		t.Fatalf("CreateSpeech: %v", err)
	}
	if resp.ContentType != "audio/ogg" {
		t.Fatalf("content-type = %q, want audio/ogg", resp.ContentType)
	}
}

func TestCreateSpeechMapsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate limit","type":"rate_limit_error","code":"rate_limit"}}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateSpeech(context.Background(), compat.SpeechRequest{Model: "tts-1", Input: "hi", Voice: "alloy"})
	if err == nil {
		t.Fatal("expected error")
	}
	compatErr, ok := err.(*compat.Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if compatErr.Status != http.StatusTooManyRequests {
		t.Fatalf("status = %d", compatErr.Status)
	}
}

func TestCreateTranscriptionForwardsMultipartRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Fatalf("content-type = %q", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if v := r.FormValue("model"); v != "whisper-1" {
			t.Fatalf("model = %q", v)
		}
		if v := r.FormValue("language"); v != "en" {
			t.Fatalf("language = %q", v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"transcribed text"}`))
	}))
	defer server.Close()

	temp := 0.5
	p := newProvider(t, server.URL+"/v1")
	resp, err := p.CreateTranscription(context.Background(), compat.AudioTranscriptionRequest{
		Model:          "whisper-1",
		File:           []byte("audio-data"),
		Filename:       "test.wav",
		Language:       "en",
		Temperature:    &temp,
	})
	if err != nil {
		t.Fatalf("CreateTranscription: %v", err)
	}
	if resp.Text != "transcribed text" {
		t.Fatalf("text = %q", resp.Text)
	}
}

func TestCreateTranslationForwardsMultipartRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/translations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		// Translation should NOT include language field
		if v := r.FormValue("language"); v != "" {
			t.Fatalf("language unexpectedly present: %q", v)
		}
		if v := r.FormValue("model"); v != "whisper-1" {
			t.Fatalf("model = %q", v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"translated text"}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	resp, err := p.CreateTranslation(context.Background(), compat.AudioTranslationRequest{
		Model:    "whisper-1",
		File:     []byte("audio-data"),
		Filename: "test.wav",
	})
	if err != nil {
		t.Fatalf("CreateTranslation: %v", err)
	}
	if resp.Text != "translated text" {
		t.Fatalf("text = %q", resp.Text)
	}
}

func TestCreateCompletionForwardsRequest(t *testing.T) {
	var got compat.CompletionsRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Fatalf("accept = %q", accept)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type = %q", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"cmpl_upstream","object":"text_completion","created":1,"model":"upstream-model","choices":[{"index":0,"text":"hello","finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	resp, err := p.CreateCompletion(context.Background(), compat.CompletionsRequest{Model: "upstream-model", Prompt: json.RawMessage(`"hello"`)})
	if err != nil {
		t.Fatalf("CreateCompletion: %v", err)
	}
	if resp.Model != "upstream-model" || len(resp.Choices) != 1 || resp.Choices[0].Text != "hello" {
		t.Fatalf("response = %#v", resp)
	}
	if got.Stream {
		t.Fatal("stream should be false")
	}
}

func TestCreateCompletionMapsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate limit","type":"rate_limit_error","code":"rate_limit"}}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateCompletion(context.Background(), compat.CompletionsRequest{Model: "upstream-model", Prompt: json.RawMessage(`"hello"`)})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*compat.Error); !ok {
		t.Fatalf("error type = %T", err)
	}
}

func TestStreamCompletionReadsSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"cmpl_upstream\",\"object\":\"text_completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"text\":\"hello\",\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	stream, err := p.StreamCompletion(context.Background(), compat.CompletionsRequest{Model: "upstream-model", Prompt: json.RawMessage(`"hello"`)})
	if err != nil {
		t.Fatalf("StreamCompletion: %v", err)
	}
	defer stream.Close()
	chunk, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("chunk: %v", err)
	}
	if chunk.Choices[0].Text != "hello" {
		t.Fatalf("text = %q", chunk.Choices[0].Text)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("end error = %v, want EOF", err)
	}
}

func TestCreateImageForwardsRequest(t *testing.T) {
	var got compat.ImageGenerationRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"created":1,"data":[{"url":"https://example.com/img.png"}]}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	resp, err := p.CreateImage(context.Background(), compat.ImageGenerationRequest{Model: "dall-e-3", Prompt: "a cat", N: intPtr(1), Size: "1024x1024"})
	if err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].URL != "https://example.com/img.png" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCreateImageMapsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"invalid size","type":"invalid_request_error","code":"invalid_size","param":"size"}}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateImage(context.Background(), compat.ImageGenerationRequest{Model: "dall-e-3", Prompt: "a cat"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*compat.Error); !ok {
		t.Fatalf("error type = %T", err)
	}
}

func TestCreateModerationForwardsRequest(t *testing.T) {
	var got compat.ModerationRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/moderations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"modr_1","model":"text-moderation-stable","results":[{"flagged":false,"categories":{},"category_scores":{}}]}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	resp, err := p.CreateModeration(context.Background(), compat.ModerationRequest{Model: "text-moderation-stable", Input: json.RawMessage(`"hello"`)})
	if err != nil {
		t.Fatalf("CreateModeration: %v", err)
	}
	if resp.ID != "modr_1" || len(resp.Results) != 1 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCreateModerationMapsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"unauthorized","type":"authentication_error","code":null}}`))
	}))
	defer server.Close()

	p := newProvider(t, server.URL+"/v1")
	_, err := p.CreateModeration(context.Background(), compat.ModerationRequest{Model: "text-moderation-stable", Input: json.RawMessage(`"hello"`)})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*compat.Error); !ok {
		t.Fatalf("error type = %T", err)
	}
}

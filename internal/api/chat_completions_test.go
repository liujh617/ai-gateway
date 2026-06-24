package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"open-ai-gateway/internal/api"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/provider/fake"
	"open-ai-gateway/internal/router"
)

const testAPIKey = "test-key"

func TestChatCompletionsNonStreamOK(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got compat.ChatCompletionResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Object != "chat.completion" {
		t.Fatalf("object = %q", got.Object)
	}
	if got.Model != "test-model" {
		t.Fatalf("model = %q", got.Model)
	}
	if len(got.Choices) != 1 || got.Choices[0].Message.Role != "assistant" {
		t.Fatalf("unexpected choices: %+v", got.Choices)
	}
}

func TestChatCompletionsStreamOK(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","stream":true,"messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q", got)
	}
	text := rr.Body.String()
	if !strings.Contains(text, "data: {") {
		t.Fatalf("missing SSE data chunk: %s", text)
	}
	if !strings.Contains(text, `"object":"chat.completion.chunk"`) {
		t.Fatalf("missing chunk object: %s", text)
	}
	if !strings.Contains(text, "data: [DONE]\n\n") {
		t.Fatalf("missing DONE event: %s", text)
	}
}

func TestChatCompletionsInvalidJSON(t *testing.T) {
	handler := newTestHandler(fake.New())

	rr := doJSON(handler, `{`, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestChatCompletionsMissingModel(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestChatCompletionsMissingMessages(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model"}`

	rr := doJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestChatCompletionsUnauthorized(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, false)

	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestChatCompletionsModelNotFound(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"unknown","messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestChatCompletionsProviderError(t *testing.T) {
	p := fake.New()
	p.Err = errors.New("upstream failed")
	handler := newTestHandler(p)
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	assertError(t, rr, http.StatusBadGateway, "server_error")
}

func TestChatCompletionsClientCancelClosesStream(t *testing.T) {
	p := newBlockingProvider()
	handler := newTestHandler(p)
	body := `{"model":"test-model","stream":true,"messages":[{"role":"user","content":"hello"}]}`

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rr, req)
		close(done)
	}()

	select {
	case <-p.stream.entered:
	case <-time.After(time.Second):
		t.Fatal("stream Next was not entered")
	}
	cancel()

	select {
	case <-p.stream.closed:
	case <-time.After(time.Second):
		t.Fatal("stream was not closed after client cancellation")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after client cancellation")
	}
}

func TestModelsOK(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"id":"test-model"`) {
		t.Fatalf("missing model: %s", rr.Body.String())
	}
}

func TestHealthzDoesNotRequireAuth(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func newTestHandler(p provider.Provider) http.Handler {
	modelRouter := router.NewModelRouter([]router.ModelRoute{{
		ExternalModel: "test-model",
		UpstreamModel: "upstream-test-model",
		Provider:      p,
	}})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return api.NewServer(modelRouter, testAPIKey, logger).Handler()
}

func doJSON(handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func assertError(t *testing.T, rr *httptest.ResponseRecorder, status int, typ string) {
	t.Helper()
	if rr.Code != status {
		t.Fatalf("status = %d, want %d, body = %s", rr.Code, status, rr.Body.String())
	}
	var got compat.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if got.Error.Type != typ {
		t.Fatalf("error type = %q, want %q", got.Error.Type, typ)
	}
}

type blockingProvider struct {
	stream *blockingStream
}

func newBlockingProvider() *blockingProvider {
	return &blockingProvider{stream: newBlockingStream()}
}

func (p *blockingProvider) ListModels(ctx context.Context) ([]compat.Model, error) {
	return nil, nil
}

func (p *blockingProvider) CreateChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func (p *blockingProvider) StreamChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return p.stream, nil
}

type blockingStream struct {
	entered chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func newBlockingStream() *blockingStream {
	return &blockingStream{
		entered: make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

func (s *blockingStream) Next(ctx context.Context) (*compat.ChatCompletionChunk, error) {
	s.once.Do(func() { close(s.entered) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s *blockingStream) Close() error {
	close(s.closed)
	return nil
}

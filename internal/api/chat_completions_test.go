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
	"open-ai-gateway/internal/middleware"
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

func TestChatCompletionsRequiresJSONContentType(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	missing := doJSONWithContentType(handler, body, true, "")
	assertError(t, missing, http.StatusUnsupportedMediaType, "invalid_request_error")

	text := doJSONWithContentType(handler, body, true, "text/plain")
	assertError(t, text, http.StatusUnsupportedMediaType, "invalid_request_error")

	withCharset := doJSONWithContentType(handler, body, true, "application/json; charset=utf-8")
	if withCharset.Code != http.StatusOK {
		t.Fatalf("charset status = %d, body = %s", withCharset.Code, withCharset.Body.String())
	}
}

func TestChatCompletionsRejectsTrailingJSON(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}{}`

	rr := doJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestChatCompletionsBodyTooLarge(t *testing.T) {
	handler := newTestHandlerWithOptions(fake.New(), api.Options{
		MaxBodyBytes: 8,
	})

	rr := doJSON(handler, `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`, true)

	assertError(t, rr, http.StatusRequestEntityTooLarge, "invalid_request_error")
}

func TestChatCompletionsPreservesExtraRequestFields(t *testing.T) {
	p := &captureProvider{}
	handler := newTestHandler(p)
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}],"tools":[{"type":"function","function":{"name":"lookup"}}],"tool_choice":"auto","response_format":{"type":"json_object"}}`

	rr := doJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if p.chatReq.Model != "upstream-test-model" {
		t.Fatalf("model = %q", p.chatReq.Model)
	}
	if string(p.chatReq.Extra["tools"]) == "" {
		t.Fatal("tools field was not preserved")
	}
	if string(p.chatReq.Extra["tool_choice"]) != `"auto"` {
		t.Fatalf("tool_choice = %s", p.chatReq.Extra["tool_choice"])
	}
	if string(p.chatReq.Extra["response_format"]) != `{"type":"json_object"}` {
		t.Fatalf("response_format = %s", p.chatReq.Extra["response_format"])
	}
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

func TestChatCompletionsProviderTimeout(t *testing.T) {
	handler := newTestHandlerWithOptions(&slowProvider{}, api.Options{
		RequestTimeout: 10 * time.Millisecond,
		StreamTimeout:  time.Second,
	})
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	assertError(t, rr, http.StatusGatewayTimeout, "server_error")
}

func TestChatCompletionsRateLimit(t *testing.T) {
	handler := newTestHandlerWithOptions(fake.New(), api.Options{
		RateLimiter: middleware.NewRateLimiter(1),
	})
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	first := doJSON(handler, body, true)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	second := doJSON(handler, body, true)

	assertError(t, second, http.StatusTooManyRequests, "rate_limit_error")
}

func TestHealthzBypassesRateLimit(t *testing.T) {
	handler := newTestHandlerWithOptions(fake.New(), api.Options{
		RateLimiter: middleware.NewRateLimiter(1),
	})
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	first := doJSON(handler, body, true)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	second := doJSON(handler, body, true)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", second.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestReadyzBypassesRateLimit(t *testing.T) {
	handler := newTestHandlerWithOptions(fake.New(), api.Options{
		RateLimiter: middleware.NewRateLimiter(1),
	})
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	first := doJSON(handler, body, true)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	second := doJSON(handler, body, true)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", second.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("readyz status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestVersionBypassesRateLimit(t *testing.T) {
	handler := newTestHandlerWithOptions(fake.New(), api.Options{
		RateLimiter: middleware.NewRateLimiter(1),
	})
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	first := doJSON(handler, body, true)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	second := doJSON(handler, body, true)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", second.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("version status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestMetricsDoesNotRequireAuth(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("content-type = %q", got)
	}
	if !strings.Contains(rr.Body.String(), "open_ai_gateway_http_requests_total") {
		t.Fatalf("missing metrics: %s", rr.Body.String())
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := newTestHandler(fake.New())

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assertNoSniff(t, health)

	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assertNoSniff(t, metrics)

	unauthorized := doJSON(handler, `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`, false)
	assertNoSniff(t, unauthorized)
}

func TestMetricsRecordsRequests(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	ok := doJSON(handler, body, true)
	if ok.Code != http.StatusOK {
		t.Fatalf("ok status = %d, body = %s", ok.Code, ok.Body.String())
	}
	bad := doJSON(handler, `{`, true)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad status = %d, body = %s", bad.Code, bad.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	text := rr.Body.String()
	for _, want := range []string{
		`open_ai_gateway_http_requests_total{method="POST",path="/v1/chat/completions",status="200"} 1`,
		`open_ai_gateway_http_requests_total{method="POST",path="/v1/chat/completions",status="400"} 1`,
		`open_ai_gateway_http_request_duration_seconds_total{method="POST",path="/v1/chat/completions",status="200"}`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("metrics missing %s: %s", want, text)
		}
	}
}

func TestMetricsBypassesRateLimit(t *testing.T) {
	handler := newTestHandlerWithOptions(fake.New(), api.Options{
		RateLimiter: middleware.NewRateLimiter(1),
	})
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	first := doJSON(handler, body, true)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	second := doJSON(handler, body, true)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", second.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestChatCompletionsStreamUsesStreamTimeout(t *testing.T) {
	handler := newTestHandlerWithOptions(&delayedStreamProvider{}, api.Options{
		RequestTimeout: time.Nanosecond,
		StreamTimeout:  time.Second,
	})
	body := `{"model":"test-model","stream":true,"messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "data: [DONE]\n\n") {
		t.Fatalf("missing DONE event: %s", rr.Body.String())
	}
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

func TestUnknownRouteReturnsJSONError(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}
}

func TestUnknownRouteRequiresAuth(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
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

func TestHealthzHeadDoesNotRequireAuth(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodHead, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("HEAD response body = %q", rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}
}

func TestReadyzDoesNotRequireAuth(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"ready"`) {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"models":1`) {
		t.Fatalf("missing model count: %s", rr.Body.String())
	}
}

func TestReadyzHeadDoesNotRequireAuth(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodHead, "/readyz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("HEAD response body = %q", rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}
}

func TestReadyzReturnsUnavailableWithoutModels(t *testing.T) {
	modelRouter := router.NewModelRouter(nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := api.NewServer(modelRouter, testAPIKey, logger).Handler()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"not_ready"`) {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestReadyzHeadReturnsUnavailableWithoutModels(t *testing.T) {
	modelRouter := router.NewModelRouter(nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := api.NewServer(modelRouter, testAPIKey, logger).Handler()
	req := httptest.NewRequest(http.MethodHead, "/readyz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("HEAD response body = %q", rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}
}

func TestVersionDoesNotRequireAuth(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	for _, want := range []string{`"version":"dev"`, `"commit":"unknown"`, `"build_time":"unknown"`} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Fatalf("version response missing %s: %s", want, rr.Body.String())
		}
	}
}

func TestEmbeddingsOK(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","input":"hello"}`

	rr := doEmbeddingsJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got compat.EmbeddingResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Object != "list" || got.Model != "test-model" {
		t.Fatalf("unexpected response: %+v", got)
	}
	if len(got.Data) != 1 || len(got.Data[0].Embedding) == 0 {
		t.Fatalf("missing embedding data: %+v", got.Data)
	}
}

func TestEmbeddingsMissingInput(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model"}`

	rr := doEmbeddingsJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestEmbeddingsRequiresJSONContentType(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","input":"hello"}`

	missing := doEmbeddingsJSONWithContentType(handler, body, true, "")
	assertError(t, missing, http.StatusUnsupportedMediaType, "invalid_request_error")

	text := doEmbeddingsJSONWithContentType(handler, body, true, "text/plain")
	assertError(t, text, http.StatusUnsupportedMediaType, "invalid_request_error")

	withCharset := doEmbeddingsJSONWithContentType(handler, body, true, "application/json; charset=utf-8")
	if withCharset.Code != http.StatusOK {
		t.Fatalf("charset status = %d, body = %s", withCharset.Code, withCharset.Body.String())
	}
}

func TestEmbeddingsRejectsTrailingJSON(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","input":"hello"}{}`

	rr := doEmbeddingsJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestEmbeddingsBodyTooLarge(t *testing.T) {
	handler := newTestHandlerWithOptions(fake.New(), api.Options{
		MaxBodyBytes: 8,
	})

	rr := doEmbeddingsJSON(handler, `{"model":"test-model","input":"hello"}`, true)

	assertError(t, rr, http.StatusRequestEntityTooLarge, "invalid_request_error")
}

func TestEmbeddingsPreservesExtraRequestFields(t *testing.T) {
	p := &captureProvider{}
	handler := newTestHandler(p)
	body := `{"model":"test-model","input":"hello","dimensions":512}`

	rr := doEmbeddingsJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if p.embeddingReq.Model != "upstream-test-model" {
		t.Fatalf("model = %q", p.embeddingReq.Model)
	}
	if string(p.embeddingReq.Extra["dimensions"]) != `512` {
		t.Fatalf("dimensions = %s", p.embeddingReq.Extra["dimensions"])
	}
}

func TestEmbeddingsModelNotFound(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"unknown","input":"hello"}`

	rr := doEmbeddingsJSON(handler, body, true)

	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestEmbeddingsProviderError(t *testing.T) {
	p := fake.New()
	p.Err = errors.New("upstream failed")
	handler := newTestHandler(p)
	body := `{"model":"test-model","input":"hello"}`

	rr := doEmbeddingsJSON(handler, body, true)

	assertError(t, rr, http.StatusBadGateway, "server_error")
}

func TestChatCompletionsRejectsEmbeddingOnlyModel(t *testing.T) {
	handler := newCapabilityTestHandler(fake.New())
	body := `{"model":"embedding-model","messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestEmbeddingsRejectsChatOnlyModel(t *testing.T) {
	handler := newCapabilityTestHandler(fake.New())
	body := `{"model":"chat-model","input":"hello"}`

	rr := doEmbeddingsJSON(handler, body, true)

	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestAccessLogIncludesRouteFields(t *testing.T) {
	var logs bytes.Buffer
	handler := newTestHandlerWithLogger(fake.New(), slog.New(slog.NewJSONHandler(&logs, nil)), api.Options{})
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	text := logs.String()
	for _, want := range []string{
		`"external_model":"test-model"`,
		`"provider":"fake-provider"`,
		`"upstream_model":"upstream-test-model"`,
		`"stream":false`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("log missing %s: %s", want, text)
		}
	}
	if strings.Contains(text, "Bearer") || strings.Contains(text, testAPIKey) {
		t.Fatalf("log leaked auth material: %s", text)
	}
}

func TestAccessLogIncludesErrorType(t *testing.T) {
	var logs bytes.Buffer
	handler := newTestHandlerWithLogger(fake.New(), slog.New(slog.NewJSONHandler(&logs, nil)), api.Options{})
	body := `{"model":"unknown","messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	text := logs.String()
	if !strings.Contains(text, `"error_type":"invalid_request_error"`) {
		t.Fatalf("log missing error type: %s", text)
	}
	if !strings.Contains(text, `"external_model":"unknown"`) {
		t.Fatalf("log missing external model: %s", text)
	}
}

func TestAccessLogIncludesAuthErrorWithoutToken(t *testing.T) {
	var logs bytes.Buffer
	handler := newTestHandlerWithLogger(fake.New(), slog.New(slog.NewJSONHandler(&logs, nil)), api.Options{})
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, false)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	text := logs.String()
	if !strings.Contains(text, `"error_type":"authentication_error"`) {
		t.Fatalf("log missing error type: %s", text)
	}
	if strings.Contains(text, "Bearer") || strings.Contains(text, testAPIKey) {
		t.Fatalf("log leaked auth material: %s", text)
	}
}

func TestAccessLogStreamStatus(t *testing.T) {
	var logs bytes.Buffer
	handler := newTestHandlerWithLogger(fake.New(), slog.New(slog.NewJSONHandler(&logs, nil)), api.Options{})
	body := `{"model":"test-model","stream":true,"messages":[{"role":"user","content":"hello"}]}`

	rr := doJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	text := logs.String()
	if !strings.Contains(text, `"status":200`) {
		t.Fatalf("log missing status: %s", text)
	}
	if !strings.Contains(text, `"stream":true`) {
		t.Fatalf("log missing stream flag: %s", text)
	}
}

func newTestHandler(p provider.Provider) http.Handler {
	return newTestHandlerWithOptions(p, api.Options{})
}

func newTestHandlerWithOptions(p provider.Provider, opts api.Options) http.Handler {
	return newTestHandlerWithLogger(p, slog.New(slog.NewTextHandler(io.Discard, nil)), opts)
}

func newTestHandlerWithLogger(p provider.Provider, logger *slog.Logger, opts api.Options) http.Handler {
	modelRouter := router.NewModelRouter([]router.ModelRoute{{
		ExternalModel: "test-model",
		UpstreamModel: "upstream-test-model",
		ProviderName:  "fake-provider",
		Provider:      p,
	}})
	return api.NewServer(modelRouter, testAPIKey, logger, opts).Handler()
}

func newCapabilityTestHandler(p provider.Provider) http.Handler {
	modelRouter := router.NewModelRouter([]router.ModelRoute{
		{
			ExternalModel: "chat-model",
			UpstreamModel: "upstream-chat-model",
			ProviderName:  "fake-provider",
			Capabilities:  map[string]bool{"chat": true},
			Provider:      p,
		},
		{
			ExternalModel: "embedding-model",
			UpstreamModel: "upstream-embedding-model",
			ProviderName:  "fake-provider",
			Capabilities:  map[string]bool{"embeddings": true},
			Provider:      p,
		},
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return api.NewServer(modelRouter, testAPIKey, logger).Handler()
}

func doJSON(handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	return doJSONWithContentType(handler, body, auth, "application/json")
}

func doJSONWithContentType(handler http.Handler, body string, auth bool, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func doEmbeddingsJSON(handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	return doEmbeddingsJSONWithContentType(handler, body, auth, "application/json")
}

func doEmbeddingsJSONWithContentType(handler http.Handler, body string, auth bool, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewBufferString(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
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

func assertNoSniff(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
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

func (p *blockingProvider) CreateEmbedding(ctx context.Context, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, errors.New("not implemented")
}

type captureProvider struct {
	chatReq      compat.ChatCompletionRequest
	embeddingReq compat.EmbeddingRequest
}

func (p *captureProvider) ListModels(ctx context.Context) ([]compat.Model, error) {
	return nil, nil
}

func (p *captureProvider) CreateChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	p.chatReq = req
	return &compat.ChatCompletionResponse{
		ID:      "chatcmpl_capture",
		Object:  "chat.completion",
		Created: 1,
		Model:   req.Model,
		Choices: []compat.ChatCompletionChoice{{
			Index: 0,
			Message: compat.ChatMessage{
				Role:    "assistant",
				Content: json.RawMessage(`"ok"`),
			},
			FinishReason: "stop",
		}},
	}, nil
}

func (p *captureProvider) StreamChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return nil, errors.New("not implemented")
}

func (p *captureProvider) CreateEmbedding(ctx context.Context, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	p.embeddingReq = req
	return &compat.EmbeddingResponse{
		Object: "list",
		Model:  req.Model,
		Data: []compat.EmbeddingData{{
			Object:    "embedding",
			Index:     0,
			Embedding: []float64{0.1, 0.2},
		}},
	}, nil
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

type slowProvider struct{}

func (p *slowProvider) ListModels(ctx context.Context) ([]compat.Model, error) {
	return nil, nil
}

func (p *slowProvider) CreateChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (p *slowProvider) StreamChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return nil, errors.New("not implemented")
}

func (p *slowProvider) CreateEmbedding(ctx context.Context, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type delayedStreamProvider struct{}

func (p *delayedStreamProvider) ListModels(ctx context.Context) ([]compat.Model, error) {
	return nil, nil
}

func (p *delayedStreamProvider) CreateChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func (p *delayedStreamProvider) StreamChatCompletion(ctx context.Context, req compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return &delayedStream{}, nil
}

func (p *delayedStreamProvider) CreateEmbedding(ctx context.Context, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, errors.New("not implemented")
}

type delayedStream struct {
	sent bool
}

func (s *delayedStream) Next(ctx context.Context) (*compat.ChatCompletionChunk, error) {
	if s.sent {
		return nil, io.EOF
	}
	select {
	case <-time.After(20 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	s.sent = true
	return &compat.ChatCompletionChunk{
		ID:      "chatcmpl_delayed",
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   "upstream-test-model",
		Choices: []compat.ChatCompletionChunkChoice{{
			Index: 0,
			Delta: compat.ChatMessageDelta{
				Content: "hello",
			},
			FinishReason: nil,
		}},
	}, nil
}

func (s *delayedStream) Close() error {
	return nil
}

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"open-ai-gateway/internal/api"
	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/provider/fake"
	"open-ai-gateway/internal/router"
)

func TestCompletionsNonStreamOK(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got compat.CompletionsResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Object != "text_completion" {
		t.Fatalf("object = %q", got.Object)
	}
	if got.Model != "test-model" {
		t.Fatalf("model = %q", got.Model)
	}
	if len(got.Choices) != 1 || got.Choices[0].Text == "" {
		t.Fatalf("unexpected choices: %+v", got.Choices)
	}
}

func TestAuditCompletionsNonStream(t *testing.T) {
	rec := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: rec})
	body := `{"model":"test-model","prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(audit.TraceIDHeader, "trace-completions-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	events := rec.Events()
	if len(events) != 2 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Event != audit.EventRequest || events[0].TraceID != "trace-completions-1" || events[0].Path != "/v1/completions" {
		t.Fatalf("request audit = %#v", events[0])
	}
	if events[0].ExternalModel != "test-model" || events[0].Client != "default" {
		t.Fatalf("request audit labels = %#v", events[0])
	}
	if !strings.Contains(string(events[0].Body), `"prompt"`) {
		t.Fatalf("request body = %s", events[0].Body)
	}
	if events[1].Event != audit.EventResponse || events[1].Status != http.StatusOK {
		t.Fatalf("response audit = %#v", events[1])
	}
	if events[1].Provider != "fake-provider" || events[1].UpstreamModel != "upstream-test-model" {
		t.Fatalf("response route labels = %#v", events[1])
	}
	if !strings.Contains(string(events[1].Body), `"text_completion"`) {
		t.Fatalf("response body = %s", events[1].Body)
	}
}

func TestCompletionsStreamOK(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","stream":true,"prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

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
	if !strings.Contains(text, `"object":"text_completion.chunk"`) {
		t.Fatalf("missing chunk object: %s", text)
	}
	if !strings.Contains(text, "data: [DONE]\n\n") {
		t.Fatalf("missing DONE event: %s", text)
	}
}

func TestAuditCompletionsStream(t *testing.T) {
	rec := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: rec})
	body := `{"model":"test-model","stream":true,"prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	events := rec.Events()
	var requestSeen, chunkSeen, doneSeen bool
	for _, event := range events {
		switch event.Event {
		case audit.EventRequest:
			requestSeen = true
			if !strings.Contains(string(event.Body), `"stream":true`) {
				t.Fatalf("request body = %s", event.Body)
			}
		case audit.EventStreamChunk:
			chunkSeen = true
			if event.Provider != "fake-provider" || event.UpstreamModel != "upstream-test-model" {
				t.Fatalf("chunk route labels = %#v", event)
			}
			if !strings.Contains(string(event.Body), `"choices"`) {
				t.Fatalf("chunk body = %s", event.Body)
			}
		case audit.EventStreamDone:
			doneSeen = true
			if event.Status != http.StatusOK || event.Provider != "fake-provider" {
				t.Fatalf("done audit = %#v", event)
			}
		}
	}
	if !requestSeen || !chunkSeen || !doneSeen {
		t.Fatalf("events = %#v", events)
	}
}

func TestCompletionsInvalidJSON(t *testing.T) {
	handler := newTestHandler(fake.New())

	rr := doCompletionsJSON(handler, `{`, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestCompletionsRequiresJSONContentType(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","prompt":"hello"}`

	missing := doCompletionsJSONWithContentType(handler, body, true, "")
	assertError(t, missing, http.StatusUnsupportedMediaType, "invalid_request_error")

	text := doCompletionsJSONWithContentType(handler, body, true, "text/plain")
	assertError(t, text, http.StatusUnsupportedMediaType, "invalid_request_error")

	withCharset := doCompletionsJSONWithContentType(handler, body, true, "application/json; charset=utf-8")
	if withCharset.Code != http.StatusOK {
		t.Fatalf("charset status = %d, body = %s", withCharset.Code, withCharset.Body.String())
	}
}

func TestCompletionsRejectsTrailingJSON(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","prompt":"hello"}{}`

	rr := doCompletionsJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestCompletionsBodyTooLarge(t *testing.T) {
	handler := newTestHandlerWithOptions(fake.New(), api.Options{
		MaxBodyBytes: 8,
	})

	rr := doCompletionsJSON(handler, `{"model":"test-model","prompt":"hello"}`, true)

	assertError(t, rr, http.StatusRequestEntityTooLarge, "invalid_request_error")
}

func TestCompletionsMissingModel(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestCompletionsMissingPrompt(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model"}`

	rr := doCompletionsJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestCompletionsEmptyPromptArray(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","prompt":[]}`

	rr := doCompletionsJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestAuditCompletionsValidationError(t *testing.T) {
	rec := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: rec})
	body := `{"model":"test-model"}`

	rr := doCompletionsJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
	events := rec.Events()
	if len(events) != 1 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Event != audit.EventError || events[0].Status != http.StatusBadRequest {
		t.Fatalf("error audit = %#v", events[0])
	}
	if events[0].Path != "/v1/completions" || events[0].ExternalModel != "test-model" {
		t.Fatalf("error audit labels = %#v", events[0])
	}
	if !strings.Contains(string(events[0].Body), `"error"`) {
		t.Fatalf("error body = %s", events[0].Body)
	}
}

func TestCompletionsUnauthorized(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"test-model","prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, false)

	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestCompletionsModelNotFound(t *testing.T) {
	handler := newTestHandler(fake.New())
	body := `{"model":"unknown","prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestCompletionsProviderError(t *testing.T) {
	p := fake.New()
	p.Err = errors.New("upstream failed")
	handler := newTestHandler(p)
	body := `{"model":"test-model","prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

	assertError(t, rr, http.StatusBadGateway, "server_error")
}

func TestCompletionsFallsBackOnProviderError(t *testing.T) {
	primary := fake.New()
	primary.Err = errors.New("upstream failed")
	fallback := &captureCompletionProvider{}
	handler := newFallbackTestHandler(primary, fallback.provider())
	body := `{"model":"test-model","prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if fallback.completionReq.Model != "backup-test-model" {
		t.Fatalf("fallback model = %q", fallback.completionReq.Model)
	}

	assertMetricsContains(t, handler, `open_ai_gateway_provider_fallbacks_total{path="/v1/completions",model="test-model",from_provider="primary-provider",to_provider="backup-provider",client="default"} 1`)
}

func TestCompletionsStreamFallsBackOnOpenError(t *testing.T) {
	primary := fake.New()
	primary.StreamErr = errors.New("stream connect failed")
	fallback := fake.New()
	handler := newFallbackTestHandler(primary, fallback)
	body := `{"model":"test-model","stream":true,"prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "data: [DONE]\n\n") {
		t.Fatalf("missing DONE event: %s", rr.Body.String())
	}

	assertMetricsContains(t, handler, `open_ai_gateway_provider_fallbacks_total{path="/v1/completions",model="test-model",from_provider="primary-provider",to_provider="backup-provider",client="default"} 1`)
}

func TestCompletionsSkipsUnhealthyProvider(t *testing.T) {
	primary := &countingCompletionProvider{err: errors.New("upstream failed")}
	fallback := &captureCompletionProvider{}
	handler := newFallbackTestHandler(primary.provider(), fallback.provider())
	body := `{"model":"test-model","prompt":"hello"}`

	for i := 0; i < 2; i++ {
		rr := doCompletionsJSON(handler, body, true)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, body = %s", i, rr.Code, rr.Body.String())
		}
	}
	if calls := primary.calls(); calls != 2 {
		t.Fatalf("primary calls before circuit = %d, want 2", calls)
	}

	rr := doCompletionsJSON(handler, body, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if calls := primary.calls(); calls != 2 {
		t.Fatalf("primary was called while unhealthy: calls = %d", calls)
	}
	assertMetricsContains(t, handler, `open_ai_gateway_provider_circuit_open_total{path="/v1/completions",model="test-model",provider="primary-provider",client="default"} 1`)
	assertMetricsContains(t, handler, `open_ai_gateway_provider_health_status{provider="primary-provider",state="healthy"} 0`)
	assertMetricsContains(t, handler, `open_ai_gateway_provider_health_status{provider="primary-provider",state="unhealthy"} 1`)
}

func TestCompletionsAllProvidersUnhealthyReturns503(t *testing.T) {
	primary := &countingCompletionProvider{err: errors.New("upstream failed")}
	fallback := &countingCompletionProvider{err: errors.New("upstream failed")}
	handler := newFallbackTestHandler(primary.provider(), fallback.provider())
	body := `{"model":"test-model","prompt":"hello"}`

	// Trigger circuit breaker on both providers.
	for i := 0; i < 2; i++ {
		rr := doCompletionsJSON(handler, body, true)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, body = %s", i, rr.Code, rr.Body.String())
		}
	}

	// Both providers should now be unhealthy; next request must return 503, not panic.
	rr := doCompletionsJSON(handler, body, true)
	assertError(t, rr, http.StatusServiceUnavailable, "server_error")
}

func TestCompletionsStreamAllProvidersUnhealthyReturns503(t *testing.T) {
	primary := &countingCompletionProvider{err: errors.New("upstream failed")}
	fallback := &countingCompletionProvider{err: errors.New("upstream failed")}
	handler := newFallbackTestHandler(primary.provider(), fallback.provider())
	body := `{"model":"test-model","stream":true,"prompt":"hello"}`

	// Trigger circuit breaker on both providers.
	for i := 0; i < 2; i++ {
		rr := doCompletionsJSON(handler, body, true)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, body = %s", i, rr.Code, rr.Body.String())
		}
	}

	rr := doCompletionsJSON(handler, body, true)
	assertError(t, rr, http.StatusServiceUnavailable, "server_error")
}

func TestCompletionsDoesNotFallBackOnInvalidRequest(t *testing.T) {
	primary := fake.New()
	primary.Err = compat.InvalidRequest("upstream rejected request", "body")
	fallback := &captureCompletionProvider{}
	handler := newFallbackTestHandler(primary, fallback.provider())
	body := `{"model":"test-model","prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
	if fallback.completionReq.Model != "" {
		t.Fatalf("fallback was called with model %q", fallback.completionReq.Model)
	}
	assertMetricsNotContains(t, handler, "open_ai_gateway_provider_fallbacks_total{")
}

func TestCompletionsProviderTimeout(t *testing.T) {
	handler := newTestHandlerWithOptions(&slowCompletionProvider{}, api.Options{
		RequestTimeout: 10 * time.Millisecond,
		StreamTimeout:  time.Second,
	})
	body := `{"model":"test-model","prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

	assertError(t, rr, http.StatusGatewayTimeout, "server_error")
}

func TestCompletionsClientCancelClosesStream(t *testing.T) {
	p := newBlockingCompletionProvider()
	handler := newTestHandler(p)
	body := `{"model":"test-model","stream":true,"prompt":"hello"}`

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBufferString(body)).WithContext(ctx)
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

func TestCompletionsRejectsChatOnlyModel(t *testing.T) {
	handler := newCapabilityTestHandler(fake.New())
	body := `{"model":"chat-model","prompt":"hello"}`

	rr := doCompletionsJSON(handler, body, true)

	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestCompletionsRejectsClientDisallowedModel(t *testing.T) {
	handler, provider := newClientModelAccessTestHandler()
	body := `{"model":"other-model","prompt":"hello"}`

	rr := doJSONWithKey(handler, body, "alpha-secret")
	_ = provider
	// Re-route to /v1/completions path by issuing a fresh request instead.
	completionsReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBufferString(body))
	completionsReq.Header.Set("Content-Type", "application/json")
	completionsReq.Header.Set("Authorization", "Bearer alpha-secret")
	completionsRR := httptest.NewRecorder()
	handler.ServeHTTP(completionsRR, completionsReq)

	assertError(t, completionsRR, http.StatusNotFound, "invalid_request_error")
}

func TestCompletionsWrongMethodReturnsJSONError(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/completions", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
	if got := rr.Header().Get("Allow"); got != "POST" {
		t.Fatalf("allow = %q", got)
	}
}

// doCompletionsJSON issues a POST /v1/completions request.
func doCompletionsJSON(handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	return doCompletionsJSONWithContentType(handler, body, auth, "application/json")
}

func doCompletionsJSONWithContentType(handler http.Handler, body string, auth bool, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBufferString(body))
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

// captureCompletionProvider captures the last CompletionsRequest it received.
type captureCompletionProvider struct {
	completionReq compat.CompletionsRequest
}

func (p *captureCompletionProvider) provider() provider.Provider {
	return p
}

func (p *captureCompletionProvider) ListModels(context.Context) ([]compat.Model, error) {
	return nil, nil
}

func (p *captureCompletionProvider) CreateChatCompletion(context.Context, compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func (p *captureCompletionProvider) StreamChatCompletion(context.Context, compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return nil, errors.New("not implemented")
}

func (p *captureCompletionProvider) CreateCompletion(_ context.Context, req compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	p.completionReq = req
	return &compat.CompletionsResponse{
		ID:      "cmpl_capture",
		Object:  "text_completion",
		Created: 1,
		Model:   req.Model,
		Choices: []compat.CompletionsChoice{{
			Text:         "ok",
			Index:        0,
			FinishReason: "stop",
		}},
	}, nil
}

func (p *captureCompletionProvider) StreamCompletion(context.Context, compat.CompletionsRequest) (provider.CompletionStream, error) {
	return nil, errors.New("not implemented")
}

func (p *captureCompletionProvider) CreateEmbedding(context.Context, compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, errors.New("not implemented")
}

// countingCompletionProvider tracks CreateCompletion calls and returns the
// configured error, used to drive circuit-breaker tests.
type countingCompletionProvider struct {
	mu       sync.Mutex
	err      error
	calls    int
	response string
}

func (p *countingCompletionProvider) provider() provider.Provider {
	return p
}

func (p *countingCompletionProvider) ListModels(context.Context) ([]compat.Model, error) {
	return nil, nil
}

func (p *countingCompletionProvider) CreateChatCompletion(context.Context, compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func (p *countingCompletionProvider) StreamChatCompletion(context.Context, compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return nil, errors.New("not implemented")
}

func (p *countingCompletionProvider) CreateCompletion(_ context.Context, req compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	p.mu.Lock()
	p.calls++
	err := p.err
	response := p.response
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if response == "" {
		response = "ok"
	}
	return &compat.CompletionsResponse{
		ID:      "cmpl_counting",
		Object:  "text_completion",
		Created: 1,
		Model:   req.Model,
		Choices: []compat.CompletionsChoice{{
			Text:         response,
			Index:        0,
			FinishReason: "stop",
		}},
	}, nil
}

func (p *countingCompletionProvider) StreamCompletion(_ context.Context, req compat.CompletionsRequest) (provider.CompletionStream, error) {
	p.mu.Lock()
	p.calls++
	err := p.err
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return fake.New().StreamCompletion(context.Background(), req)
}

func (p *countingCompletionProvider) CreateEmbedding(context.Context, compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, errors.New("not implemented")
}

func (p *countingCompletionProvider) calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// slowCompletionProvider blocks until the request context is canceled, used to
// trigger provider timeouts.
type slowCompletionProvider struct{}

func (p *slowCompletionProvider) ListModels(context.Context) ([]compat.Model, error) { return nil, nil }

func (p *slowCompletionProvider) CreateChatCompletion(context.Context, compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func (p *slowCompletionProvider) StreamChatCompletion(context.Context, compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return nil, errors.New("not implemented")
}

func (p *slowCompletionProvider) CreateCompletion(ctx context.Context, _ compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (p *slowCompletionProvider) StreamCompletion(context.Context, compat.CompletionsRequest) (provider.CompletionStream, error) {
	return nil, errors.New("not implemented")
}

func (p *slowCompletionProvider) CreateEmbedding(context.Context, compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, errors.New("not implemented")
}

// blockingCompletionProvider exposes a stream that blocks in Next until the
// request context is canceled, so tests can verify the stream is closed.
type blockingCompletionProvider struct {
	stream *blockingCompletionStream
}

func newBlockingCompletionProvider() *blockingCompletionProvider {
	return &blockingCompletionProvider{stream: newBlockingCompletionStream()}
}

func (p *blockingCompletionProvider) ListModels(context.Context) ([]compat.Model, error) {
	return nil, nil
}

func (p *blockingCompletionProvider) CreateChatCompletion(context.Context, compat.ChatCompletionRequest) (*compat.ChatCompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func (p *blockingCompletionProvider) StreamChatCompletion(context.Context, compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return nil, errors.New("not implemented")
}

func (p *blockingCompletionProvider) CreateCompletion(context.Context, compat.CompletionsRequest) (*compat.CompletionsResponse, error) {
	return nil, errors.New("not implemented")
}

func (p *blockingCompletionProvider) StreamCompletion(context.Context, compat.CompletionsRequest) (provider.CompletionStream, error) {
	return p.stream, nil
}

func (p *blockingCompletionProvider) CreateEmbedding(context.Context, compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	return nil, errors.New("not implemented")
}

type blockingCompletionStream struct {
	entered chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func newBlockingCompletionStream() *blockingCompletionStream {
	return &blockingCompletionStream{
		entered: make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

func (s *blockingCompletionStream) Next(ctx context.Context) (*compat.CompletionsChunk, error) {
	s.once.Do(func() { close(s.entered) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s *blockingCompletionStream) Close() error {
	close(s.closed)
	return nil
}

// ensure unused imports are referenced.
var _ = io.EOF
var _ = middleware.NewRateLimiter
var _ = router.TokenPricing{}

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"open-ai-gateway/internal/api"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/provider/deepseek"
	providerdialect "open-ai-gateway/internal/provider/dialect"
	"open-ai-gateway/internal/provider/fake"
	"open-ai-gateway/internal/reasoningenvelope"
	"open-ai-gateway/internal/router"
)

func TestResponsesDeepSeekStreamSealsReasoningBeforeFunctionCall(t *testing.T) {
	upstream := &deepseekStreamingProvider{Provider: fake.New()}
	registry := providerdialect.NewRegistry()
	if err := registry.Register(providerdialect.NewOpenAICompatible()); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(deepseek.NewDialectWithIDGenerator(func() (string, error) { return "env_stream", nil })); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	codec, err := reasoningenvelope.New(reasoningenvelope.Config{
		ActiveKey: reasoningenvelope.Key{ID: "active", Bytes: bytes.Repeat([]byte{5}, 32)},
		TTL:       time.Hour, MaxEnvelopeBytes: 1 << 20, MaxPlaintextBytes: 1 << 19, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	modelRouter := router.NewModelRouter([]router.ModelRoute{{
		ExternalModel: "codex-model", UpstreamModel: "deepseek-chat", ProviderName: "deepseek-primary",
		Dialect: "deepseek", ReasoningReplay: true, Capabilities: map[string]bool{"chat": true}, Provider: upstream,
	}})
	auditRecorder := &memoryAuditRecorder{}
	handler := api.NewServer(modelRouter, testAPIKey, slog.New(slog.NewTextHandler(io.Discard, nil)), api.Options{
		Dialects: registry, ReasoningEnvelope: codec, ReasoningAudience: "test-audience", Audit: auditRecorder,
	}).Handler()

	recorder := doResponsesJSON(handler, `{"model":"codex-model","input":"weather","stream":true,"store":false}`, true)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status=%d content-type=%q body=%s", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "private reasoning") {
		t.Fatalf("raw reasoning leaked into SSE: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]\n\n") {
		t.Fatalf("missing stream sentinel: %s", body)
	}
	reasoningDone := strings.Index(body, `"type":"reasoning"`)
	functionAdded := strings.Index(body, `"type":"function_call"`)
	if reasoningDone < 0 || functionAdded <= reasoningDone {
		t.Fatalf("reasoning item was not emitted before function call: %s", body)
	}
	completed := completedResponseFromSSE(t, body)
	if len(completed.Output) != 3 || completed.Output[0].Type != "reasoning" || completed.Output[1].Type != "message" || completed.Output[2].Type != "function_call" {
		t.Fatalf("completed output=%#v", completed.Output)
	}
	payload, err := codec.Open(completed.Output[0].EncryptedContent, reasoningenvelope.Binding{Audience: "test-audience", Client: "default", ExternalModel: "codex-model"})
	if err != nil {
		t.Fatal(err)
	}
	if payload.ReasoningContent != "private reasoning" || len(payload.CallIDs) != 1 || payload.CallIDs[0] != "call_1" {
		t.Fatalf("payload=%#v", payload)
	}
	if !upstream.closed {
		t.Fatal("upstream stream was not closed")
	}
	for _, event := range auditRecorder.Events() {
		if strings.Contains(string(event.Body), "private reasoning") {
			t.Fatalf("raw reasoning leaked into audit event %q: %s", event.Event, event.Body)
		}
	}
}

func TestResponsesDialectStreamCancellationClosesUpstreamWithoutCompletion(t *testing.T) {
	upstream := &cancelStreamingProvider{Provider: fake.New(), started: make(chan struct{})}
	modelRouter := router.NewModelRouter([]router.ModelRoute{{
		ExternalModel: "test-model", UpstreamModel: "upstream-test-model", ProviderName: "fake-provider", Provider: upstream,
	}})
	handler := api.NewServer(modelRouter, testAPIKey, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"test-model","input":"hello","stream":true}`)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+testAPIKey)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(recorder, request)
		close(done)
	}()
	<-upstream.started
	cancel()
	<-done
	if !upstream.closed {
		t.Fatal("upstream stream was not closed after cancellation")
	}
	if strings.Contains(recorder.Body.String(), "response.completed") || strings.Contains(recorder.Body.String(), "[DONE]") {
		t.Fatalf("cancelled stream completed: %s", recorder.Body.String())
	}
}

type deepseekStreamingProvider struct {
	*fake.Provider
	closed bool
}

type cancelStreamingProvider struct {
	*fake.Provider
	started chan struct{}
	closed  bool
}

func (p *cancelStreamingProvider) StreamChatCompletion(context.Context, compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	return &cancelResponseStream{provider: p}, nil
}

type cancelResponseStream struct {
	provider *cancelStreamingProvider
	once     bool
}

func (s *cancelResponseStream) Next(ctx context.Context) (*compat.ChatCompletionChunk, error) {
	if !s.once {
		s.once = true
		close(s.provider.started)
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s *cancelResponseStream) Close() error {
	s.provider.closed = true
	return nil
}

func (p *deepseekStreamingProvider) StreamChatCompletion(context.Context, compat.ChatCompletionRequest) (provider.ChatCompletionStream, error) {
	finish := "tool_calls"
	chunks := []*compat.ChatCompletionChunk{
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Extra: map[string]json.RawMessage{
			"reasoning_content": json.RawMessage(`"private "`),
		}}}}},
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Content: "checking", Extra: map[string]json.RawMessage{
			"reasoning_content": json.RawMessage(`"reasoning"`),
			"tool_calls":        json.RawMessage(`[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{"}}]`),
		}}}}},
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, Delta: compat.ChatMessageDelta{Extra: map[string]json.RawMessage{
			"tool_calls": json.RawMessage(`[{"index":0,"function":{"arguments":"}"}}]`),
		}}}}},
		{Choices: []compat.ChatCompletionChunkChoice{{Index: 0, FinishReason: &finish}}},
		{Usage: &compat.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}},
	}
	return &deepseekReasoningStream{provider: p, chunks: chunks}, nil
}

type deepseekReasoningStream struct {
	provider *deepseekStreamingProvider
	chunks   []*compat.ChatCompletionChunk
	index    int
}

func (s *deepseekReasoningStream) Next(context.Context) (*compat.ChatCompletionChunk, error) {
	if s.index == len(s.chunks) {
		return nil, io.EOF
	}
	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

func (s *deepseekReasoningStream) Close() error {
	s.provider.closed = true
	return nil
}

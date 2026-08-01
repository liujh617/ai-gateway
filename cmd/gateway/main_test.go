package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/config"
)

func TestRunAuditInspectSummarizesEventsWithoutBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-trace.jsonl")
	events := []audit.Event{
		{
			Timestamp:     time.Date(2026, 7, 11, 1, 2, 3, 0, time.UTC),
			Event:         audit.EventRequest,
			RequestID:     "req_1",
			TraceID:       "trace_1",
			Path:          "/v1/chat/completions",
			Client:        "default",
			ExternalModel: "test-model",
			Body:          json.RawMessage(`{"messages":[{"role":"user","content":"secret prompt"}]}`),
		},
		{
			Timestamp:     time.Date(2026, 7, 11, 1, 2, 4, 0, time.UTC),
			Event:         audit.EventResponse,
			RequestID:     "req_1",
			TraceID:       "trace_1",
			Path:          "/v1/chat/completions",
			Client:        "default",
			ExternalModel: "test-model",
			Provider:      "fake",
			UpstreamModel: "upstream-test-model",
			Status:        200,
			Body:          json.RawMessage(`{"choices":[{"message":{"content":"secret completion"}}]}`),
		},
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create audit file: %v", err)
	}
	for _, event := range events {
		if err := json.NewEncoder(file).Encode(event); err != nil {
			t.Fatalf("write audit event: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close audit file: %v", err)
	}

	var out bytes.Buffer
	if err := runAuditInspect(&out, path); err != nil {
		t.Fatalf("runAuditInspect: %v", err)
	}

	text := out.String()
	if strings.Contains(text, "secret prompt") || strings.Contains(text, "secret completion") {
		t.Fatalf("inspect leaked body: %s", text)
	}
	for _, want := range []string{
		`"event":"request"`,
		`"event":"response"`,
		`"request_id":"req_1"`,
		`"trace_id":"trace_1"`,
		`"body_bytes":`,
		`"status":200`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("inspect missing %s: %s", want, text)
		}
	}
}

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

func TestBuildRouterPreservesReasoningDialectMetadata(t *testing.T) {
	cfg := config.Default()
	cfg.ReasoningEnvelope = config.ReasoningEnvelopeConfig{
		Enabled: true, Audience: "test", TTLSeconds: 60,
		MaxEnvelopeBytes: 1024, MaxPlaintextBytes: 512,
		MaxItemsPerRequest: 16, MaxToolCallsPerTurn: 4,
		ActiveKey: config.ReasoningEnvelopeKeyConfig{ID: "active", KeyEnv: "TEST_REASONING_KEY"},
	}
	t.Setenv("TEST_REASONING_KEY", "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE")
	cfg.Models = map[string]config.ModelConfig{
		"codex-model": {
			Provider: "fake", UpstreamModel: "deepseek-chat", Dialect: "deepseek", ReasoningReplay: true,
			Capabilities: []string{"chat"},
			Fallbacks:    []config.ModelFallbackConfig{{Provider: "fake", UpstreamModel: "fallback-chat", Dialect: "deepseek", ReasoningReplay: true}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	modelRouter, err := buildRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	route, compatErr := modelRouter.Resolve("codex-model")
	if compatErr != nil {
		t.Fatal(compatErr)
	}
	attempts := route.Attempts()
	if len(attempts) != 2 || attempts[0].Dialect != "deepseek" || !attempts[0].ReasoningReplay || attempts[1].Dialect != "deepseek" || !attempts[1].ReasoningReplay {
		t.Fatalf("attempts=%#v", attempts)
	}
}

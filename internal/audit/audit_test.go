package audit_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/requestctx"
)

func TestJSONLRecorderWritesEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit", "agent.jsonl")
	rec, err := audit.NewJSONLRecorder(path)
	if err != nil {
		t.Fatalf("NewJSONLRecorder: %v", err)
	}
	defer rec.Close()

	body := json.RawMessage(`{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`)
	rec.Record(context.Background(), audit.Event{
		Timestamp:     time.Date(2026, 7, 11, 1, 2, 3, 4, time.UTC),
		Event:         audit.EventRequest,
		RequestID:     "req_1",
		TraceID:       "trace_1",
		Path:          "/v1/chat/completions",
		Client:        "alpha",
		ExternalModel: "test-model",
		Provider:      "fake-provider",
		UpstreamModel: "upstream-test-model",
		Body:          body,
	})

	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("missing audit line")
	}
	var got map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatalf("decode audit line: %v", err)
	}
	if got["event"] != "request" || got["request_id"] != "req_1" || got["trace_id"] != "trace_1" {
		t.Fatalf("audit event = %#v", got)
	}
	bodyMap, ok := got["body"].(map[string]any)
	if !ok || bodyMap["model"] != "test-model" {
		t.Fatalf("body = %#v", got["body"])
	}
	if scanner.Scan() {
		t.Fatalf("unexpected second audit line: %s", scanner.Text())
	}
}

func TestNoopRecorderDoesNotCreateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.jsonl")
	var rec audit.NoopRecorder
	rec.Record(context.Background(), audit.Event{Event: audit.EventRequest, Body: json.RawMessage(`{"ok":true}`)})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stat = %v, want not exist", err)
	}
}

func TestTraceIDFromRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req = req.WithContext(requestctx.WithRequestID(req.Context(), "req_fallback"))
	if got := audit.TraceIDFromRequest(req); got != "req_fallback" {
		t.Fatalf("trace id fallback = %q", got)
	}
	req.Header.Set(audit.TraceIDHeader, " agent-session-1 ")
	if got := audit.TraceIDFromRequest(req); got != "agent-session-1" {
		t.Fatalf("trace id header = %q", got)
	}
}

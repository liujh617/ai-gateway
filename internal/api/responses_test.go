package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"open-ai-gateway/internal/api"
	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/fake"
)

func TestResponsesNonStreamOK(t *testing.T) {
	rr := doResponsesJSON(newTestHandler(fake.New()), `{"model":"test-model","input":"hello"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var got compat.Response
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Object != "response" || got.Model != "test-model" || len(got.Output) != 1 || got.Output[0].Content[0].Text != "Hello from open-ai-gateway." {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestResponsesRejectsUnsupportedField(t *testing.T) {
	rr := doResponsesJSON(newTestHandler(fake.New()), `{"model":"test-model","input":"hello","tools":[]}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestResponsesStreamOK(t *testing.T) {
	rr := doResponsesJSON(newTestHandler(fake.New()), `{"model":"test-model","input":"hello","stream":true}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type=%q", got)
	}
	text := rr.Body.String()
	wantOrder := []string{"response.created", "response.in_progress", "response.output_item.added", "response.content_part.added", "response.output_text.delta", "response.output_text.done", "response.content_part.done", "response.output_item.done", "response.completed"}
	last := -1
	for _, event := range wantOrder {
		index := strings.Index(text, "event: "+event+"\n")
		if index <= last {
			t.Fatalf("event %q missing or out of order: %s", event, text)
		}
		last = index
	}
	if strings.Contains(text, "[DONE]") {
		t.Fatalf("unexpected chat sentinel: %s", text)
	}
}

func TestResponsesAuditUsesResponsesBodies(t *testing.T) {
	recorder := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: recorder})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"test-model","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set(audit.TraceIDHeader, "responses-trace")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	events := recorder.Events()
	if len(events) != 2 || events[0].Path != "/v1/responses" || events[1].Path != "/v1/responses" {
		t.Fatalf("events=%#v", events)
	}
	if !strings.Contains(string(events[0].Body), `"input":"hello"`) {
		t.Fatalf("request body=%s", events[0].Body)
	}
	if !strings.Contains(string(events[1].Body), `"object":"response"`) {
		t.Fatalf("response body=%s", events[1].Body)
	}
}

func TestResponsesMetricsUseResponsesPath(t *testing.T) {
	handler := newTestHandler(fake.New())
	rr := doResponsesJSON(handler, `{"model":"test-model","input":"hello"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	assertMetricsContains(t, handler, `open_ai_gateway_tokens_total{path="/v1/responses",model="test-model",provider="fake-provider",type="total",client="default"} 2`)
}

func TestResponsesStreamAuditsTypedEvents(t *testing.T) {
	recorder := &memoryAuditRecorder{}
	handler := newTestHandlerWithOptions(fake.New(), api.Options{Audit: recorder})
	rr := doResponsesJSON(handler, `{"model":"test-model","input":"hello","stream":true}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	foundDelta := false
	for _, event := range recorder.Events() {
		if event.Event == audit.EventStreamChunk && strings.Contains(string(event.Body), `"type":"response.output_text.delta"`) {
			foundDelta = true
		}
	}
	if !foundDelta {
		t.Fatalf("typed delta not audited: %#v", recorder.Events())
	}
}

func doResponsesJSON(handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

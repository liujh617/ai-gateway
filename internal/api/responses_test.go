package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

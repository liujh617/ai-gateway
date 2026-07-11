package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestChatCompletion(t *testing.T) {
	rec := serveModelRequest(t, "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", "application/json", `{"model":"chat-deployment","messages":[{"role":"user","content":"hello"}]}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"object":"chat.completion"`) {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestStreamingChatCompletion(t *testing.T) {
	rec := serveModelRequest(t, "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", "text/event-stream", `{"model":"chat-deployment","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "data: [DONE]") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestEmbedding(t *testing.T) {
	rec := serveModelRequest(t, "/openai/deployments/embedding-deployment/embeddings?api-version=2024-02-15-preview", "application/json", `{"model":"embedding-deployment","input":"hello"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"object":"list"`) {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestRejectsInvalidAzureContract(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		apiKey        string
		authorization string
		contentType   string
		accept        string
		body          string
	}{
		{name: "api version", path: "/openai/deployments/chat-deployment/chat/completions?api-version=wrong", apiKey: "local-azure-test-key", contentType: "application/json", accept: "application/json", body: `{"model":"chat-deployment"}`},
		{name: "api key", path: "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", apiKey: "wrong", contentType: "application/json", accept: "application/json", body: `{"model":"chat-deployment"}`},
		{name: "authorization", path: "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", apiKey: "local-azure-test-key", authorization: "Bearer forbidden", contentType: "application/json", accept: "application/json", body: `{"model":"chat-deployment"}`},
		{name: "content type", path: "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", apiKey: "local-azure-test-key", contentType: "text/plain", accept: "application/json", body: `{"model":"chat-deployment"}`},
		{name: "accept", path: "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", apiKey: "local-azure-test-key", contentType: "application/json", accept: "text/plain", body: `{"model":"chat-deployment"}`},
		{name: "model", path: "/openai/deployments/chat-deployment/chat/completions?api-version=2024-02-15-preview", apiKey: "local-azure-test-key", contentType: "application/json", accept: "application/json", body: `{"model":"wrong"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("api-key", tt.apiKey)
			req.Header.Set("Authorization", tt.authorization)
			req.Header.Set("Content-Type", tt.contentType)
			req.Header.Set("Accept", tt.accept)
			rec := httptest.NewRecorder()
			newHandler().ServeHTTP(rec, req)
			if rec.Code < 400 {
				t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func serveModelRequest(t *testing.T, path, accept, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("api-key", "local-azure-test-key")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, req)
	return rec
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"open-ai-gateway/internal/requestctx"
)

func TestRequestIDReusesValidHeader(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestIDFromContext(r.Context()); got != "client-request-1" {
			t.Fatalf("context request id = %q", got)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(requestctx.RequestIDHeader, "client-request-1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get(requestctx.RequestIDHeader); got != "client-request-1" {
		t.Fatalf("response request id = %q", got)
	}
}

func TestRequestIDTrimsHeader(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestIDFromContext(r.Context()); got != "client-request-2" {
			t.Fatalf("context request id = %q", got)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(requestctx.RequestIDHeader, "  client-request-2  ")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get(requestctx.RequestIDHeader); got != "client-request-2" {
		t.Fatalf("response request id = %q", got)
	}
}

func TestRequestIDGeneratesWhenHeaderInvalid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "internal whitespace", raw: "client request"},
		{name: "too long", raw: strings.Repeat("a", maxRequestIDLength+1)},
		{name: "non ascii", raw: "请求-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var contextID string
			handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contextID = RequestIDFromContext(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			req.Header.Set(requestctx.RequestIDHeader, tt.raw)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			responseID := rr.Header().Get(requestctx.RequestIDHeader)
			if responseID == "" {
				t.Fatal("missing response request id")
			}
			if responseID == tt.raw {
				t.Fatalf("invalid request id was reused: %q", responseID)
			}
			if contextID != responseID {
				t.Fatalf("context request id = %q, response id = %q", contextID, responseID)
			}
		})
	}
}

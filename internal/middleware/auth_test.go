package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"open-ai-gateway/internal/compat"
)

func TestAuthAcceptsAnyConfiguredAPIKey(t *testing.T) {
	handler := Auth([]string{"first-key", "second-key"}, testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, key := range []string{"first-key", "second-key"} {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer "+key)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("key %q status = %d, body = %s", key, rr.Code, rr.Body.String())
		}
	}
}

func TestAuthRejectsUnknownAPIKey(t *testing.T) {
	handler := Auth([]string{"first-key", "second-key"}, testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestAuthAllowsPublicRoutesWithoutAPIKey(t *testing.T) {
	handler := Auth([]string{"first-key"}, testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

type testErrorWriter struct{}

func (testErrorWriter) WriteError(w http.ResponseWriter, err *compat.Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)
	_ = json.NewEncoder(w).Encode(compat.ErrorResponseFor(err))
}

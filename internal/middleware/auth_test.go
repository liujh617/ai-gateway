package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"open-ai-gateway/internal/compat"
)

func TestAuthAcceptsAnyConfiguredAPIKey(t *testing.T) {
	handler := Auth([]AuthCredential{
		{Client: "first", APIKey: "first-key"},
		{Client: "second", APIKey: "second-key"},
	}, testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	handler := Auth([]AuthCredential{
		{Client: "first", APIKey: "first-key"},
		{Client: "second", APIKey: "second-key"},
	}, testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	handler := Auth([]AuthCredential{{Client: "first", APIKey: "first-key"}}, testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestAuthSetsLogClientForPublicRoute(t *testing.T) {
	var logs bytes.Buffer
	handler := Logging(slog.New(slog.NewJSONHandler(&logs, nil)))(Auth([]AuthCredential{{Client: "first", APIKey: "first-key"}}, testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assertLogContainsClient(t, logs.String(), "public")
}

func TestAuthSetsLogClientForUnconfiguredAuth(t *testing.T) {
	var logs bytes.Buffer
	handler := Logging(slog.New(slog.NewJSONHandler(&logs, nil)))(Auth(nil, testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assertLogContainsClient(t, logs.String(), "unconfigured")
}

func TestAuthSetsLogClientForAuthenticationFailure(t *testing.T) {
	var logs bytes.Buffer
	handler := Logging(slog.New(slog.NewJSONHandler(&logs, nil)))(Auth([]AuthCredential{{Client: "first", APIKey: "first-key"}}, testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assertLogContainsClient(t, logs.String(), "unauthenticated")
}

func assertLogContainsClient(t *testing.T, logs string, client string) {
	t.Helper()
	want := `"client":"` + client + `"`
	if !strings.Contains(logs, want) {
		t.Fatalf("log missing %s: %s", want, logs)
	}
}

type testErrorWriter struct{}

func (testErrorWriter) WriteError(w http.ResponseWriter, err *compat.Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)
	_ = json.NewEncoder(w).Encode(compat.ErrorResponseFor(err))
}

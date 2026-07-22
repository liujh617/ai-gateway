package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"open-ai-gateway/internal/api"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/fake"
	"open-ai-gateway/internal/router"
)

func TestModerationsOK(t *testing.T) {
	rr := doModerations(t, newTestHandler(fake.New()), `{"model":"test-model","input":"hello"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type=%q", ct)
	}
	var resp compat.ModerationResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Flagged {
		t.Fatalf("response=%#v", resp)
	}
}

func TestModerationsMissingModel(t *testing.T) {
	rr := doModerations(t, newTestHandler(fake.New()), `{"input":"hello"}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestModerationsEmptyInput(t *testing.T) {
	rr := doModerations(t, newTestHandler(fake.New()), `{"model":"test-model","input":""}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestModerationsRequiresAuth(t *testing.T) {
	rr := doModerations(t, newTestHandler(fake.New()), `{"model":"test-model","input":"hello"}`, false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestModerationsWrongCapability(t *testing.T) {
	modelRouter := router.NewModelRouter([]router.ModelRoute{
		{ExternalModel: "test-model", UpstreamModel: "test-model", ProviderName: "fake", Provider: fake.New(), Capabilities: map[string]bool{"chat": true}},
	})
	handler := api.NewServer(modelRouter, testAPIKey, nil).Handler()
	rr := doModerations(t, handler, `{"model":"test-model","input":"hello"}`, true)
	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestModerationsMethodNotAllowed(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/moderations", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
	if got := rr.Header().Get("Allow"); got != "POST" {
		t.Fatalf("Allow=%q", got)
	}
}

func doModerations(t *testing.T, handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/moderations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

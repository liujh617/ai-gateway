package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"open-ai-gateway/internal/provider/fake"
)

func TestRealtimeMissingModel(t *testing.T) {
	rr := doRealtimeRequest(newTestHandler(fake.New()), "/v1/realtime", true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestRealtimeRequiresAuth(t *testing.T) {
	rr := doRealtimeRequest(newTestHandler(fake.New()), "/v1/realtime?model=test-model", false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestRealtimeWrongCapability(t *testing.T) {
	rr := doRealtimeRequest(newTestHandler(fake.New()), "/v1/realtime?model=test-model", true)
	// fake provider has no realtime capability by default, so expect 501 not implemented
	if rr.Code != http.StatusNotImplemented && rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func doRealtimeRequest(handler http.Handler, path string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

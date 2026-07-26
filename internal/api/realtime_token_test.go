package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"open-ai-gateway/internal/provider/fake"
)

type realtimeTokenResponse struct {
	ClientSecret string `json:"client_secret"`
	ExpiresAt    int64  `json:"expires_at"`
	TTLSeconds   int    `json:"ttl_seconds"`
}

func TestRealtimeTokenCreate(t *testing.T) {
	rr := doRealtimeTokenRequest(newTestHandler(fake.New()), `{"model":"test-model"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp realtimeTokenResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ClientSecret == "" || resp.TTLSeconds != 300 {
		t.Fatalf("response=%#v", resp)
	}
}

func TestRealtimeTokenMissingModel(t *testing.T) {
	rr := doRealtimeTokenRequest(newTestHandler(fake.New()), `{}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestRealtimeTokenRequiresAuth(t *testing.T) {
	rr := doRealtimeTokenRequest(newTestHandler(fake.New()), `{"model":"test-model"}`, false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func doRealtimeTokenRequest(handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/realtime/tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

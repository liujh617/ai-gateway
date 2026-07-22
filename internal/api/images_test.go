package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"open-ai-gateway/internal/api"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/provider/fake"
	"open-ai-gateway/internal/router"
)

func TestImageGenerationsOK(t *testing.T) {
	rr := doImageGenerations(t, newTestHandler(fake.New()), `{"model":"test-model","prompt":"a cat"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type=%q", ct)
	}
	var resp compat.ImageGenerationResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 || resp.Data[0].URL != "https://example.com/fake-image.png" {
		t.Fatalf("response=%#v", resp)
	}
}

func TestImageGenerationsMissingModel(t *testing.T) {
	rr := doImageGenerations(t, newTestHandler(fake.New()), `{"prompt":"a cat"}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestImageGenerationsEmptyPrompt(t *testing.T) {
	rr := doImageGenerations(t, newTestHandler(fake.New()), `{"model":"test-model","prompt":""}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestImageGenerationsRequiresAuth(t *testing.T) {
	rr := doImageGenerations(t, newTestHandler(fake.New()), `{"model":"test-model","prompt":"a cat"}`, false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestImageGenerationsModelNotFound(t *testing.T) {
	handler := newTestHandlerWithOptions(fake.New(), api.Options{
		ClientModels: map[string][]string{"default": {"other-model"}},
	})
	rr := doImageGenerations(t, handler, `{"model":"test-model","prompt":"a cat"}`, true)
	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestImageGenerationsWrongCapability(t *testing.T) {
	modelRouter := router.NewModelRouter([]router.ModelRoute{
		{ExternalModel: "test-model", UpstreamModel: "test-model", ProviderName: "fake", Provider: fake.New(), Capabilities: map[string]bool{"chat": true}},
	})
	handler := api.NewServer(modelRouter, testAPIKey, nil).Handler()
	rr := doImageGenerations(t, handler, `{"model":"test-model","prompt":"a cat"}`, true)
	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestImageGenerationsMethodNotAllowed(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/images/generations", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
	if got := rr.Header().Get("Allow"); got != "POST" {
		t.Fatalf("Allow=%q", got)
	}
}

func TestImageGenerationsProviderError(t *testing.T) {
	p := fake.New()
	p.Err = provider.ErrStreamClosed
	rr := doImageGenerations(t, newTestHandler(p), `{"model":"test-model","prompt":"a cat"}`, true)
	if rr.Code >= 200 && rr.Code < 300 {
		t.Fatalf("expected error status, got %d", rr.Code)
	}
}

func TestImageGenerationsFallback(t *testing.T) {
	primary := fake.New()
	primary.Err = errors.New("upstream failed")
	fallback := fake.New()
	modelRouter := router.NewModelRouter([]router.ModelRoute{
		{ExternalModel: "test-model", UpstreamModel: "test-model", ProviderName: "primary", Provider: primary, Capabilities: map[string]bool{"images": true}, Fallbacks: []router.ProviderRoute{
			{ProviderName: "fallback", Provider: fallback, UpstreamModel: "test-model"},
		}},
	})
	handler := api.NewServer(modelRouter, testAPIKey, nil).Handler()
	rr := doImageGenerations(t, handler, `{"model":"test-model","prompt":"a cat"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func doImageGenerations(t *testing.T, handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

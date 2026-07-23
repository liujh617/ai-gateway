package api_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
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

func TestAudioTranscriptionsOK(t *testing.T) {
	rr := doAudioTranscriptions(t, newTestHandler(fake.New()), "hello", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type=%q", ct)
	}
	var resp compat.AudioTranscriptionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Text == "" {
		t.Fatalf("empty text in response")
	}
}

func TestAudioTranscriptionsMissingModel(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := newAudioFormRequest("/v1/audio/transcriptions", map[string]string{}, []byte("hello"), "test.wav")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestAudioTranscriptionsMissingFile(t *testing.T) {
	handler := newTestHandler(fake.New())
	b, contentType := newFormData(map[string]string{"model": "test-model"}, nil, "")
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestAudioTranscriptionsRequiresAuth(t *testing.T) {
	rr := doAudioTranscriptions(t, newTestHandler(fake.New()), "hello", false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestAudioTranscriptionsWrongCapability(t *testing.T) {
	modelRouter := router.NewModelRouter([]router.ModelRoute{
		{ExternalModel: "test-model", UpstreamModel: "test-model", ProviderName: "fake", Provider: fake.New(), Capabilities: map[string]bool{"chat": true}},
	})
	handler := api.NewServer(modelRouter, testAPIKey, nil).Handler()
	rr := doAudioTranscriptions(t, handler, "hello", true)
	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestAudioTranscriptionsMethodNotAllowed(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/audio/transcriptions", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
	if got := rr.Header().Get("Allow"); got != "POST" {
		t.Fatalf("Allow=%q", got)
	}
}

func TestAudioTranscriptionsProviderError(t *testing.T) {
	p := fake.New()
	p.Err = provider.ErrStreamClosed
	rr := doAudioTranscriptions(t, newTestHandler(p), "hello", true)
	if rr.Code >= 200 && rr.Code < 300 {
		t.Fatalf("expected error status, got %d", rr.Code)
	}
}

func TestAudioTranscriptionsFallback(t *testing.T) {
	primary := fake.New()
	primary.Err = provider.ErrStreamClosed
	fallback := fake.New()
	modelRouter := router.NewModelRouter([]router.ModelRoute{
		{ExternalModel: "test-model", UpstreamModel: "test-model", ProviderName: "primary", Provider: primary, Capabilities: map[string]bool{"transcriptions": true}, Fallbacks: []router.ProviderRoute{
			{ProviderName: "fallback", Provider: fallback, UpstreamModel: "test-model"},
		}},
	})
	handler := api.NewServer(modelRouter, testAPIKey, nil).Handler()
	rr := doAudioTranscriptions(t, handler, "hello", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAudioTranslationsOK(t *testing.T) {
	rr := doAudioTranslations(t, newTestHandler(fake.New()), "hello", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type=%q", ct)
	}
	var resp compat.AudioTranslationResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Text == "" {
		t.Fatalf("empty text in response")
	}
}

func TestAudioTranslationsRequiresAuth(t *testing.T) {
	rr := doAudioTranslations(t, newTestHandler(fake.New()), "hello", false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestAudioTranslationsWrongCapability(t *testing.T) {
	modelRouter := router.NewModelRouter([]router.ModelRoute{
		{ExternalModel: "test-model", UpstreamModel: "test-model", ProviderName: "fake", Provider: fake.New(), Capabilities: map[string]bool{"chat": true}},
	})
	handler := api.NewServer(modelRouter, testAPIKey, nil).Handler()
	rr := doAudioTranslations(t, handler, "hello", true)
	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestAudioSpeechOK(t *testing.T) {
	rr := doAudioSpeech(t, newTestHandler(fake.New()), `{"model":"test-model","input":"hello","voice":"alloy"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Fatalf("Content-Type=%q", ct)
	}
	if rr.Body.String() != "fake-audio-data" {
		t.Fatalf("body=%q", rr.Body.String())
	}
}

func TestAudioSpeechMissingModel(t *testing.T) {
	rr := doAudioSpeech(t, newTestHandler(fake.New()), `{"input":"hello","voice":"alloy"}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestAudioSpeechMissingInput(t *testing.T) {
	rr := doAudioSpeech(t, newTestHandler(fake.New()), `{"model":"test-model","voice":"alloy"}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestAudioSpeechMissingVoice(t *testing.T) {
	rr := doAudioSpeech(t, newTestHandler(fake.New()), `{"model":"test-model","input":"hello"}`, true)
	assertError(t, rr, http.StatusBadRequest, "invalid_request_error")
}

func TestAudioSpeechRequiresAuth(t *testing.T) {
	rr := doAudioSpeech(t, newTestHandler(fake.New()), `{"model":"test-model","input":"hello","voice":"alloy"}`, false)
	assertError(t, rr, http.StatusUnauthorized, "authentication_error")
}

func TestAudioSpeechWrongCapability(t *testing.T) {
	modelRouter := router.NewModelRouter([]router.ModelRoute{
		{ExternalModel: "test-model", UpstreamModel: "test-model", ProviderName: "fake", Provider: fake.New(), Capabilities: map[string]bool{"chat": true}},
	})
	handler := api.NewServer(modelRouter, testAPIKey, nil).Handler()
	rr := doAudioSpeech(t, handler, `{"model":"test-model","input":"hello","voice":"alloy"}`, true)
	assertError(t, rr, http.StatusNotFound, "invalid_request_error")
}

func TestAudioSpeechMethodNotAllowed(t *testing.T) {
	handler := newTestHandler(fake.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/audio/speech", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assertError(t, rr, http.StatusMethodNotAllowed, "invalid_request_error")
	if got := rr.Header().Get("Allow"); got != "POST" {
		t.Fatalf("Allow=%q", got)
	}
}

func TestAudioSpeechProviderError(t *testing.T) {
	p := fake.New()
	p.Err = provider.ErrStreamClosed
	rr := doAudioSpeech(t, newTestHandler(p), `{"model":"test-model","input":"hello","voice":"alloy"}`, true)
	if rr.Code >= 200 && rr.Code < 300 {
		t.Fatalf("expected error status, got %d", rr.Code)
	}
}

func doAudioTranscriptions(t *testing.T, handler http.Handler, content string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	return doAudioFormRequest(t, handler, "/v1/audio/transcriptions", content, auth)
}

func doAudioTranslations(t *testing.T, handler http.Handler, content string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	return doAudioFormRequest(t, handler, "/v1/audio/translations", content, auth)
}

func doAudioFormRequest(t *testing.T, handler http.Handler, path, content string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := newAudioFormRequest(path, map[string]string{"model": "test-model"}, []byte(content), "test.wav")
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func newAudioFormRequest(path string, fields map[string]string, fileData []byte, filename string) *http.Request {
	b, ct := newFormData(fields, fileData, filename)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", ct)
	return req
}

func newFormData(fields map[string]string, fileData []byte, filename string) ([]byte, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	if fileData != nil {
		fw, _ := w.CreateFormFile("file", filename)
		_, _ = fw.Write(fileData)
	}
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

func doAudioSpeech(t *testing.T, handler http.Handler, body string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

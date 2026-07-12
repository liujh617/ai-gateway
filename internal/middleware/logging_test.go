package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingRecordsPreviousResponseBooleanOnly(t *testing.T) {
	var logs bytes.Buffer
	handler := Logging(slog.New(slog.NewJSONHandler(&logs, nil)))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetLogPreviousResponse(r.Context(), true)
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if !strings.Contains(logs.String(), `"previous_response":true`) {
		t.Fatalf("log=%s", logs.String())
	}
	if strings.Contains(logs.String(), "resp_secret") {
		t.Fatalf("log leaked response id: %s", logs.String())
	}
}

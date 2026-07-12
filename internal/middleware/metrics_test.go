package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/responsestore"
)

func TestMetricsWritesResponseStoreSnapshot(t *testing.T) {
	store := responsestore.New(responsestore.Config{TTL: time.Hour, MaxEntries: 1, MaxContextBytes: 1024, MaxTotalBytes: 2048}, nil)
	message := []compat.ChatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}}
	if err := store.Put(responsestore.Record{ID: "resp_1", Client: "alpha", Model: "m", Transcript: message}); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(responsestore.Record{ID: "resp_2", Client: "alpha", Model: "m", Transcript: message}); err != nil {
		t.Fatal(err)
	}
	store.Get("missing", "alpha", "m")
	store.Get("resp_2", "beta", "m")
	store.Get("resp_2", "alpha", "other")

	metrics := NewMetrics()
	metrics.SetResponseStore(store)
	rr := httptest.NewRecorder()
	metrics.WritePrometheus(rr)
	text := rr.Body.String()
	for _, want := range []string{
		"open_ai_gateway_response_store_entries 1",
		"open_ai_gateway_response_store_bytes ",
		`open_ai_gateway_response_store_evictions_total{reason="capacity"} 1`,
		`open_ai_gateway_response_store_misses_total{reason="not_found"} 1`,
		`open_ai_gateway_response_store_misses_total{reason="client"} 1`,
		`open_ai_gateway_response_store_misses_total{reason="model"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "resp_1") || strings.Contains(text, "resp_2") || strings.Contains(text, "alpha") {
		t.Fatalf("metrics leaked high-cardinality state: %s", text)
	}
}

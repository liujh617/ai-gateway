package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterUsesClientOverrides(t *testing.T) {
	limiter := NewClientRateLimiter(10, map[string]int{"alpha": 1})
	handler := Auth([]AuthCredential{
		{Client: "alpha", APIKey: "alpha-key"},
		{Client: "beta", APIKey: "beta-key"},
	}, testErrorWriter{})(limiter.Middleware(testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	first := performLimitedRequest(handler, "alpha-key")
	if first.Code != http.StatusNoContent {
		t.Fatalf("first alpha status = %d, body = %s", first.Code, first.Body.String())
	}
	second := performLimitedRequest(handler, "alpha-key")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second alpha status = %d, body = %s", second.Code, second.Body.String())
	}
	beta := performLimitedRequest(handler, "beta-key")
	if beta.Code != http.StatusNoContent {
		t.Fatalf("beta status = %d, body = %s", beta.Code, beta.Body.String())
	}
}

func TestRateLimiterClientOverrideCanDisableLimit(t *testing.T) {
	limiter := NewClientRateLimiter(1, map[string]int{"alpha": 0})
	handler := Auth([]AuthCredential{
		{Client: "alpha", APIKey: "alpha-key"},
		{Client: "beta", APIKey: "beta-key"},
	}, testErrorWriter{})(limiter.Middleware(testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	for i := 0; i < 2; i++ {
		rr := performLimitedRequest(handler, "alpha-key")
		if rr.Code != http.StatusNoContent {
			t.Fatalf("alpha request %d status = %d, body = %s", i+1, rr.Code, rr.Body.String())
		}
	}
	firstBeta := performLimitedRequest(handler, "beta-key")
	if firstBeta.Code != http.StatusNoContent {
		t.Fatalf("first beta status = %d, body = %s", firstBeta.Code, firstBeta.Body.String())
	}
	secondBeta := performLimitedRequest(handler, "beta-key")
	if secondBeta.Code != http.StatusTooManyRequests {
		t.Fatalf("second beta status = %d, body = %s", secondBeta.Code, secondBeta.Body.String())
	}
}

func TestRateLimiterPrunesExpiredBuckets(t *testing.T) {
	limiter := NewRateLimiter(1)
	now := time.Unix(120, 0)

	limiter.buckets["old"] = bucket{start: now.Add(-2 * time.Minute), count: 1}
	limiter.buckets["recent"] = bucket{start: now.Add(-30 * time.Second), count: 1}

	if allowed, _ := limiter.allow("current", 1, now); !allowed {
		t.Fatal("current request was not allowed")
	}

	if _, ok := limiter.buckets["old"]; ok {
		t.Fatal("expired bucket was not pruned")
	}
	if _, ok := limiter.buckets["recent"]; !ok {
		t.Fatal("recent bucket was pruned")
	}
	if _, ok := limiter.buckets["current"]; !ok {
		t.Fatal("current bucket was not recorded")
	}
}

func TestRateLimiterReturnsRetryAfterForRejectedRequest(t *testing.T) {
	limiter := NewRateLimiter(1)
	start := time.Unix(120, 0)

	if allowed, retryAfter := limiter.allow("client:alpha", 1, start); !allowed || retryAfter != 0 {
		t.Fatalf("first request allowed = %t, retryAfter = %s", allowed, retryAfter)
	}
	allowed, retryAfter := limiter.allow("client:alpha", 1, start.Add(45*time.Second))

	if allowed {
		t.Fatal("second request was allowed")
	}
	if retryAfter != 15*time.Second {
		t.Fatalf("retryAfter = %s, want 15s", retryAfter)
	}
}

func TestRateLimiterMiddlewareReturnsRemainingRetryAfter(t *testing.T) {
	limiter := NewRateLimiter(1)
	now := time.Unix(120, 0)
	limiter.now = func() time.Time {
		return now
	}
	handler := Auth([]AuthCredential{
		{Client: "alpha", APIKey: "alpha-key"},
	}, testErrorWriter{})(limiter.Middleware(testErrorWriter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	first := performLimitedRequest(handler, "alpha-key")
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	now = now.Add(45 * time.Second)
	second := performLimitedRequest(handler, "alpha-key")

	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, body = %s", second.Code, second.Body.String())
	}
	if got := second.Header().Get("Retry-After"); got != "15" {
		t.Fatalf("Retry-After = %q, want 15", got)
	}
}

func performLimitedRequest(handler http.Handler, apiKey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

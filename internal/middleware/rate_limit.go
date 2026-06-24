package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"open-ai-gateway/internal/compat"
)

type RateLimiter struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	buckets map[string]bucket
}

type bucket struct {
	start time.Time
	count int
}

func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		limit:   requestsPerMinute,
		window:  time.Minute,
		buckets: make(map[string]bucket),
	}
}

func (l *RateLimiter) Middleware(errors ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" || l == nil || l.limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			if !l.allow(rateLimitKey(r), time.Now()) {
				SetLogError(r.Context(), "rate_limit_error", nil)
				errors.WriteError(w, compat.RateLimit("rate limit exceeded"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *RateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	current := l.buckets[key]
	if current.start.IsZero() || now.Sub(current.start) >= l.window {
		l.buckets[key] = bucket{start: now, count: 1}
		return true
	}
	if current.count >= l.limit {
		return false
	}
	current.count++
	l.buckets[key] = current
	return true
}

func rateLimitKey(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "anonymous"
}

package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/routes"
)

type RateLimiter struct {
	defaultLimit int
	clientLimits map[string]int
	window       time.Duration
	observer     RateLimitRejectionObserver

	mu      sync.Mutex
	buckets map[string]bucket

	lastCleanup time.Time
}

type bucket struct {
	start time.Time
	count int
}

type RateLimitRejectionObserver interface {
	ObserveRateLimitRejection(path, client string)
}

func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return NewClientRateLimiter(requestsPerMinute, nil)
}

func NewClientRateLimiter(defaultRequestsPerMinute int, clientLimits map[string]int) *RateLimiter {
	limits := make(map[string]int, len(clientLimits))
	for client, limit := range clientLimits {
		if client != "" {
			limits[client] = limit
		}
	}
	return &RateLimiter{
		defaultLimit: defaultRequestsPerMinute,
		clientLimits: limits,
		window:       time.Minute,
		buckets:      make(map[string]bucket),
	}
}

func (l *RateLimiter) SetRejectionObserver(observer RateLimitRejectionObserver) {
	if l == nil {
		return
	}
	l.observer = observer
}

func (l *RateLimiter) Middleware(errors ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if routes.IsPublicPath(r.URL.Path) || l == nil {
				next.ServeHTTP(w, r)
				return
			}
			limit := l.limitFor(r)
			if limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			if !l.allow(rateLimitKey(r), limit, time.Now()) {
				SetLogError(r.Context(), "rate_limit_error", nil)
				l.observeRejection(r)
				errors.WriteError(w, compat.RateLimit("rate limit exceeded"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *RateLimiter) limitFor(r *http.Request) int {
	if client := ClientFromContext(r.Context()); client != "" {
		if limit, ok := l.clientLimits[client]; ok {
			return limit
		}
	}
	return l.defaultLimit
}

func (l *RateLimiter) allow(key string, limit int, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pruneExpiredBuckets(now)

	current := l.buckets[key]
	if current.start.IsZero() || now.Sub(current.start) >= l.window {
		l.buckets[key] = bucket{start: now, count: 1}
		return true
	}
	if current.count >= limit {
		return false
	}
	current.count++
	l.buckets[key] = current
	return true
}

func (l *RateLimiter) observeRejection(r *http.Request) {
	if l.observer == nil {
		return
	}
	client := ClientFromContext(r.Context())
	if client == "" {
		client = "unconfigured"
	}
	l.observer.ObserveRateLimitRejection(r.URL.Path, client)
}

func (l *RateLimiter) pruneExpiredBuckets(now time.Time) {
	if !l.lastCleanup.IsZero() && now.Sub(l.lastCleanup) < l.window {
		return
	}
	for key, current := range l.buckets {
		if now.Sub(current.start) >= l.window {
			delete(l.buckets, key)
		}
	}
	l.lastCleanup = now
}

func rateLimitKey(r *http.Request) string {
	if client := ClientFromContext(r.Context()); client != "" {
		return "client:" + client
	}
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

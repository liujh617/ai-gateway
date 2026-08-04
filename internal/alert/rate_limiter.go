package alert

import (
	"sync"
	"time"
)

// rateLimiter implements token bucket algorithm
type rateLimiter struct {
	rate       int // tokens per minute
	burst      int // max tokens
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(maxPerMinute, burst int) *rateLimiter {
	return &rateLimiter{
		rate:       maxPerMinute,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
}

// Allow checks if a request is allowed
func (r *rateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Calculate tokens to add based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastUpdate).Seconds()
	r.lastUpdate = now

	// Add tokens (rate per minute = rate/60 per second)
	tokensToAdd := float64(r.rate) / 60.0 * elapsed
	r.tokens += tokensToAdd

	// Cap at burst size
	if r.tokens > float64(r.burst) {
		r.tokens = float64(r.burst)
	}

	// Check if we have tokens
	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return true
	}

	return false
}
package wsproxy

import "sync"

// ConnectionLimiter controls the maximum number of concurrent WebSocket connections.
type ConnectionLimiter struct {
	mu    sync.Mutex
	count int
	max   int
}

// NewConnectionLimiter creates a limiter with the given maximum connections.
func NewConnectionLimiter(max int) *ConnectionLimiter {
	return &ConnectionLimiter{max: max}
}

// Acquire attempts to reserve a connection slot. Returns false if at capacity.
func (l *ConnectionLimiter) Acquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.max > 0 && l.count >= l.max {
		return false
	}
	l.count++
	return true
}

// Release frees a connection slot.
func (l *ConnectionLimiter) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.count > 0 {
		l.count--
	}
}

// Count returns the current number of active connections.
func (l *ConnectionLimiter) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count
}

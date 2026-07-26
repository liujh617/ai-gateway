package wsproxy

import (
	"sync"
	"time"
)

// ClientQuota tracks per-client connection counts and rate limits.
type ClientQuota struct {
	mu            sync.Mutex
	clients       map[string]*clientState
	maxPerClient  int
	maxPerMinute  int
}

type clientState struct {
	connections  int
	windowStart  time.Time
	windowCount  int
}

// ClientQuotaConfig configures per-client limits.
type ClientQuotaConfig struct {
	MaxPerClient int // max concurrent connections per client, 0 = unlimited
	MaxPerMinute int // max new connections per minute per client, 0 = unlimited
}

// NewClientQuota creates a per-client quota tracker.
func NewClientQuota(cfg ClientQuotaConfig) *ClientQuota {
	return &ClientQuota{
		clients:      make(map[string]*clientState),
		maxPerClient: cfg.MaxPerClient,
		maxPerMinute: cfg.MaxPerMinute,
	}
}

// Allow checks if the client can establish a new connection.
func (q *ClientQuota) Allow(client string) bool {
	if q == nil {
		return true
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	state := q.clients[client]
	if state == nil {
		state = &clientState{windowStart: time.Now()}
		q.clients[client] = state
	}
	now := time.Now()
	if now.Sub(state.windowStart) > time.Minute {
		state.windowStart = now
		state.windowCount = 0
	}
	if q.maxPerMinute > 0 && state.windowCount >= q.maxPerMinute {
		return false
	}
	if q.maxPerClient > 0 && state.connections >= q.maxPerClient {
		return false
	}
	state.connections++
	state.windowCount++
	return true
}

// Release frees a client connection slot.
func (q *ClientQuota) Release(client string) {
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	state := q.clients[client]
	if state != nil && state.connections > 0 {
		state.connections--
	}
}

package api

import (
	"sync"
	"time"
)

type ProviderHealthOptions struct {
	FailureThreshold int
	Cooldown         time.Duration
}

type providerHealth struct {
	mu               sync.RWMutex
	failureThreshold int
	cooldown         time.Duration
	now              func() time.Time
	states           map[string]*providerHealthState
}

type providerHealthState struct {
	consecutiveFailures int
	unhealthyUntil      time.Time
}

type providerHealthSnapshot struct {
	Provider string
	Healthy  bool
}

func newProviderHealth(options ProviderHealthOptions) *providerHealth {
	if options.FailureThreshold <= 0 {
		options.FailureThreshold = 2
	}
	if options.Cooldown <= 0 {
		options.Cooldown = 30 * time.Second
	}
	return &providerHealth{
		failureThreshold: options.FailureThreshold,
		cooldown:         options.Cooldown,
		now:              time.Now,
		states:           make(map[string]*providerHealthState),
	}
}

func (h *providerHealth) Register(provider string) {
	if h == nil || provider == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stateLocked(provider)
}

func (h *providerHealth) Healthy(provider string) bool {
	if h == nil || provider == "" {
		return true
	}
	h.mu.RLock()
	state := h.states[provider]
	if state != nil {
		unhealthyUntil := state.unhealthyUntil
		if unhealthyUntil.IsZero() {
			h.mu.RUnlock()
			return true
		}
		if h.now().Before(unhealthyUntil) {
			h.mu.RUnlock()
			return false
		}
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()
	state = h.stateLocked(provider)
	now := h.now()
	if state.unhealthyUntil.IsZero() {
		return true
	}
	if !now.Before(state.unhealthyUntil) {
		state.unhealthyUntil = time.Time{}
		state.consecutiveFailures = 0
		return true
	}
	return false
}

func (h *providerHealth) MarkSuccess(provider string) {
	if h == nil || provider == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	state := h.stateLocked(provider)
	state.consecutiveFailures = 0
	state.unhealthyUntil = time.Time{}
}

func (h *providerHealth) MarkFailure(provider string) bool {
	if h == nil || provider == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	state := h.stateLocked(provider)
	state.consecutiveFailures++
	if state.consecutiveFailures < h.failureThreshold {
		return false
	}
	state.unhealthyUntil = h.now().Add(h.cooldown)
	return true
}

func (h *providerHealth) Snapshot() []providerHealthSnapshot {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]providerHealthSnapshot, 0, len(h.states))
	now := h.now()
	for provider, state := range h.states {
		expired := !state.unhealthyUntil.IsZero() && !now.Before(state.unhealthyUntil)
		if expired {
			state.unhealthyUntil = time.Time{}
			state.consecutiveFailures = 0
		}
		healthy := state.unhealthyUntil.IsZero()
		out = append(out, providerHealthSnapshot{
			Provider: provider,
			Healthy:  healthy,
		})
	}
	return out
}

func (h *providerHealth) stateLocked(provider string) *providerHealthState {
	state := h.states[provider]
	if state == nil {
		state = &providerHealthState{}
		h.states[provider] = state
	}
	return state
}

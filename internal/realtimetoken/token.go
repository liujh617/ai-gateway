package realtoken

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// TokenRecord stores token metadata.
type TokenRecord struct {
	Client    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Store manages short-lived realtime access tokens.
type Store struct {
	mu     sync.Mutex
	tokens map[string]TokenRecord
}

// NewStore creates a token store with background expiry cleanup.
func NewStore() *Store {
	s := &Store{tokens: make(map[string]TokenRecord)}
	go s.cleanupLoop()
	return s
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, rec := range s.tokens {
			if now.After(rec.ExpiresAt) {
				delete(s.tokens, id)
			}
		}
		s.mu.Unlock()
	}
}

// Issue creates a new token valid for the given TTL (capped at 3600 seconds).
func (s *Store) Issue(client string, ttlSeconds int) string {
	if ttlSeconds <= 0 || ttlSeconds > 3600 {
		ttlSeconds = 300
	}
	var value [16]byte
	rand.Read(value[:])
	token := hex.EncodeToString(value[:])
	now := time.Now()
	s.mu.Lock()
	s.tokens[token] = TokenRecord{Client: client, CreatedAt: now, ExpiresAt: now.Add(time.Duration(ttlSeconds) * time.Second)}
	s.mu.Unlock()
	return token
}

// Validate checks if a token is valid and returns the associated client.
func (s *Store) Validate(token string) (client string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, found := s.tokens[token]
	if !found || time.Now().After(rec.ExpiresAt) {
		return "", false
	}
	return rec.Client, true
}

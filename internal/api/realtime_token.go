package api

import (
	"encoding/json"
	"net/http"
	"time"

	"open-ai-gateway/internal/compat"
)

type realtimeTokenRequest struct {
	Model      string `json:"model"`
	TTLSeconds int    `json:"ttl_seconds,omitempty"`
}

type realtimeTokenResponse struct {
	ClientSecret string `json:"client_secret"`
	ExpiresAt    int64  `json:"expires_at"`
	TTLSeconds   int    `json:"ttl_seconds"`
}

func (s *Server) handleRealtimeTokens(w http.ResponseWriter, r *http.Request) {
	if err := requireJSONContentType(r); err != nil {
		s.writeError(w, r, err)
		return
	}
	var req realtimeTokenRequest
	if err := decodeJSONBody(s.requestBody(w, r), &req); err != nil {
		s.writeError(w, r, decodeError(err))
		return
	}
	if req.Model == "" {
		s.writeError(w, r, compat.InvalidRequest("missing required field: model", "model"))
		return
	}

	// Validate the model has realtime capability
	_, resolveErr := s.router.ResolveByCapability("realtime")
	if resolveErr != nil {
		s.writeError(w, r, resolveErr)
		return
	}

	client := clientFromContext(r.Context())
	ttl := req.TTLSeconds
	if ttl <= 0 || ttl > 3600 {
		ttl = 300
	}
	secret := s.realtimeTokens.Issue(client, ttl)
	expiresAt := time.Now().Add(time.Duration(ttl) * time.Second).Unix()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(realtimeTokenResponse{
		ClientSecret: secret,
		ExpiresAt:    expiresAt,
		TTLSeconds:   ttl,
	})
}

// authenticateRealtimeToken checks the token query parameter and returns the client.
func (s *Server) authenticateRealtimeToken(r *http.Request) (string, bool) {
	token := r.URL.Query().Get("token")
	if token == "" {
		return "", false
	}
	return s.realtimeTokens.Validate(token)
}

// realtimeClientQuota checks and acquires client-specific connection quota.
func (s *Server) realtimeClientQuota(client string) bool {
	if s.realtimeClientQuotas == nil {
		return true
	}
	return s.realtimeClientQuotas.Allow(client)
}

func (s *Server) realtimeClientQuotaRelease(client string) {
	if s.realtimeClientQuotas != nil {
		s.realtimeClientQuotas.Release(client)
	}
}

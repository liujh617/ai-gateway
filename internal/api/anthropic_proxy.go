package api

import (
	"bytes"
	"io"
	"net/http"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
)

// handleAnthropicMessages transparently proxies Anthropic-native /v1/messages
// requests to the upstream provider. Uses x-api-key header for auth since
// Anthropic SDK clients send it instead of Authorization: Bearer.
func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	// Auth via x-api-key header (Anthropic SDK standard)
	apiKey := r.Header.Get("x-api-key")
	client := ""
	for _, cred := range s.credentials {
		if cred.APIKey == apiKey {
			client = cred.Client
			break
		}
	}
	if client == "" {
		s.writeError(w, r, compat.NewError(http.StatusUnauthorized, "authentication_error", "invalid x-api-key", nil))
		return
	}

	model := extractModelFromBody(r)
	if model == "" {
		s.writeError(w, r, compat.InvalidRequest("missing model in request body", "model"))
		return
	}

	route, err := s.router.ResolveFor(model, "chat")
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	rp, ok := route.Provider.(provider.AnthropicProxy)
	if !ok {
		s.writeError(w, r, compat.ServerError(http.StatusNotImplemented, "provider does not support Anthropic proxy"))
		return
	}

	upstreamResp, proxyErr := rp.ProxyAnthropicRequest(r)
	if proxyErr != nil {
		s.logger.Warn("anthropic proxy failed", "error", proxyErr)
		s.writeError(w, r, compat.ServerError(http.StatusBadGateway, proxyErr.Error()))
		return
	}
	defer upstreamResp.Body.Close()

	for key, values := range upstreamResp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(upstreamResp.StatusCode)
	io.Copy(w, upstreamResp.Body)
}

func extractModelFromBody(r *http.Request) string {
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	body := string(bodyBytes)

	const key = `"model":"`
	for i := 0; i <= len(body)-len(key); i++ {
		if body[i:i+len(key)] == key {
			start := i + len(key)
			for j := start; j < len(body); j++ {
				if body[j] == '"' {
					return body[start:j]
				}
			}
		}
	}
	return ""
}

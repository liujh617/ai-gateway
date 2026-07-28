package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/routes"
)

const maxAuditBodyBytes = 1 << 20 // 1MB

// handleAnthropicMessages transparently proxies Anthropic-native /v1/messages
// requests to the upstream provider. Accepts both x-api-key and Authorization:
// Bearer headers for authentication.
func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("x-api-key")
	if apiKey == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}
	}
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

	model, bodyBytes := extractModelFromBody(r)
	path := routes.NormalizePath(r.URL.Path)
	if model == "" {
		s.writeAuditedError(w, r, path, "", compat.InvalidRequest("missing model in request body", "model"))
		return
	}

	isStream := bodyHasStream(bodyBytes)

	route, resolveErr := s.router.ResolveFor(model, "chat")
	if resolveErr != nil {
		s.writeAuditedError(w, r, path, model, resolveErr)
		return
	}

	rp, ok := route.Provider.(provider.AnthropicProxy)
	if !ok {
		s.writeAuditedError(w, r, path, model,
			compat.ServerError(http.StatusNotImplemented, "provider does not support Anthropic proxy"))
		return
	}

	// Update log and metrics context with resolved client and route info.
	r = r.WithContext(middleware.WithClient(r.Context(), client))
	middleware.SetLogClient(r.Context(), client)
	middleware.SetMetricsClient(r.Context(), client)
	middleware.SetLogRoute(r.Context(), model, route.ProviderName, route.UpstreamModel)

	// Record request audit event.
	requestEvent := s.auditBaseEvent(r, audit.EventRequest, path, model)
	requestEvent.Provider = route.ProviderName
	requestEvent.UpstreamModel = route.UpstreamModel
	requestEvent.Body = bodyBytes
	s.audit.Record(r.Context(), requestEvent)

	// Apply stream timeout for streaming requests so long-running SSE
	// connections are bounded by the configured limit, not the provider's
	// short HTTP client timeout.
	if isStream {
		ctx, cancel := context.WithTimeout(r.Context(), s.streamTimeout)
		defer cancel()
		r = r.WithContext(ctx)
	}

	// Proxy request upstream.
	upstreamResp, proxyErr := rp.ProxyAnthropicRequest(r)
	if proxyErr != nil {
		s.logger.Warn("anthropic proxy failed", "error", proxyErr)
		s.writeAuditedError(w, r, path, model,
			compat.ServerError(http.StatusBadGateway, proxyErr.Error()))
		return
	}
	defer upstreamResp.Body.Close()

	// Copy response headers to client.
	for key, values := range upstreamResp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(upstreamResp.StatusCode)

	// Stream response body to client with flush, capturing audit body.
	s.copyAnthropicResponse(w, r, path, model, route.ProviderName, route.UpstreamModel, upstreamResp)
}

// copyAnthropicResponse streams the upstream response body to the client via
// chunked write + flush, while capturing up to maxAuditBodyBytes for a single
// EventResponse audit entry. Works for both SSE streaming and non-streaming responses.
func (s *Server) copyAnthropicResponse(w http.ResponseWriter, r *http.Request, path, model, providerName, upstreamModel string, upstreamResp *http.Response) {
	flusher, canFlush := w.(http.Flusher)
	auditWriter := &cappedWriter{cap: maxAuditBodyBytes}
	teeReader := io.TeeReader(upstreamResp.Body, auditWriter)

	buf := make([]byte, 32*1024)
	var streamErr error
	for {
		n, readErr := teeReader.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				streamErr = writeErr
				break
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				streamErr = readErr
			}
			break
		}
	}

	if streamErr != nil {
		s.logger.Warn("anthropic proxy stream error", "error", streamErr)
		errorEvent := s.auditBaseEvent(r, audit.EventError, path, model)
		errorEvent.Provider = providerName
		errorEvent.UpstreamModel = upstreamModel
		errorEvent.Body = auditWriter.bytes()
		errorEvent.Error = "stream_error"
		s.audit.Record(r.Context(), errorEvent)
		return
	}

	responseEvent := s.auditBaseEvent(r, audit.EventResponse, path, model)
	responseEvent.Provider = providerName
	responseEvent.UpstreamModel = upstreamModel
	responseEvent.Status = upstreamResp.StatusCode
	responseEvent.Body = auditWriter.bytes()
	s.audit.Record(r.Context(), responseEvent)
}

// bodyHasStream checks whether the request body has "stream":true at the
// top level of the JSON object.
func bodyHasStream(body []byte) bool {
	var v struct {
		Stream bool `json:"stream"`
	}
	return json.Unmarshal(body, &v) == nil && v.Stream
}

// cappedWriter collects writes up to cap bytes, then silently discards.
// It always returns the full len(p) so io.TeeReader never sees an error.
type cappedWriter struct {
	buf bytes.Buffer
	cap int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	remaining := w.cap - w.buf.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		w.buf.Write(p[:remaining])
	} else {
		w.buf.Write(p)
	}
	return len(p), nil
}

func (w *cappedWriter) bytes() []byte {
	return w.buf.Bytes()
}

func extractModelFromBody(r *http.Request) (string, []byte) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return "", nil
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	body := string(bodyBytes)

	const key = `"model":"`
	for i := 0; i <= len(body)-len(key); i++ {
		if body[i:i+len(key)] == key {
			start := i + len(key)
			for j := start; j < len(body); j++ {
				if body[j] == '"' {
					return body[start:j], bodyBytes
				}
			}
		}
	}
	return "", bodyBytes
}

package api

import (
	"net/http"
	"time"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/requestctx"
	"open-ai-gateway/internal/routes"
	"open-ai-gateway/internal/wsproxy"

	"github.com/coder/websocket"
)

// defaultRealtimeConfig is the fallback configuration when no Server config is provided.
var defaultRealtimeConfig = wsproxy.Config{
	MaxMessageBytes: 1 << 20, // 1 MB
	MaxDuration:     30 * time.Minute,
}

// realtimeLimiter controls concurrent realtime connections.
var realtimeLimiter = wsproxy.NewConnectionLimiter(100)

func (s *Server) handleRealtime(w http.ResponseWriter, r *http.Request) {
	model := r.URL.Query().Get("model")
	if model == "" {
		s.writeError(w, r, compat.InvalidRequest("missing model query parameter", "model"))
		return
	}

	cfg := s.realtimeConfig
	if cfg == nil {
		cfg = &defaultRealtimeConfig
	}

	if len(cfg.AllowedOrigins) > 0 {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, o := range cfg.AllowedOrigins {
			if o == origin {
				allowed = true
				break
			}
		}
		if !allowed {
			s.writeError(w, r, compat.NewError(http.StatusForbidden, "invalid_request_error", "origin not allowed", nil))
			return
		}
	}

	route, err := s.router.ResolveByCapability("realtime")
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	if !realtimeLimiter.Acquire() {
		s.writeError(w, r, compat.ServerError(http.StatusServiceUnavailable, "too many realtime connections"))
		return
	}
	defer realtimeLimiter.Release()

	rp, ok := route.Provider.(provider.RealtimeProvider)
	if !ok || rp.RealtimeEndpoint(model) == "" {
		s.writeError(w, r, compat.ServerError(http.StatusNotImplemented, "provider does not support realtime"))
		return
	}

	clientConn, err2 := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: cfg.AllowedOrigins,
	})
	if err2 != nil {
		s.logger.Warn("failed to accept websocket", "error", err2)
		s.wsErrorEvent(r, routes.RealtimePath, model, "ws_accept_failed")
		return
	}
	defer clientConn.CloseNow()

	upstreamURL := rp.RealtimeEndpoint(model)
	upstreamConn, _, err2 := websocket.Dial(r.Context(), upstreamURL, nil)
	if err2 != nil {
		s.logger.Warn("failed to dial upstream websocket", "url", upstreamURL, "error", err2)
		clientConn.Close(websocket.StatusInternalError, "upstream connection failed")
		s.wsErrorEvent(r, routes.RealtimePath, model, "upstream_dial_failed")
		return
	}
	defer upstreamConn.CloseNow()

	connectEvent := s.auditBaseEvent(r, audit.EventRequest, routes.RealtimePath, model)
	connectEvent.Event = "ws_connect"
	s.audit.Record(r.Context(), connectEvent)

	ctx := r.Context()
	stats := wsproxy.Relay(ctx, clientConn, upstreamConn, *cfg)

	s.logger.Info("realtime connection closed",
		"model", model,
		"provider", route.ProviderName,
		"duration_ms", stats.Duration.Milliseconds(),
		"client_bytes", stats.ClientSentBytes,
		"upstream_bytes", stats.UpstreamSentBytes,
	)

	disconnectEvent := s.auditBaseEvent(r, audit.EventRequest, routes.RealtimePath, model)
	disconnectEvent.Event = "ws_disconnect"
	disconnectEvent.DurationMS = stats.Duration.Milliseconds()
	s.audit.Record(r.Context(), disconnectEvent)
}

func (s *Server) wsErrorEvent(r *http.Request, path, model, errorMsg string) {
	s.audit.Record(r.Context(), audit.Event{
		Timestamp: time.Now().UTC(),
		Event:     audit.EventError,
		RequestID: requestctx.RequestID(r.Context()),
		TraceID:   audit.TraceIDFromRequest(r),
		Path:      path,
		Error:     errorMsg,
	})
}

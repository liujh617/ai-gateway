package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/router"
)

type Server struct {
	router *router.ModelRouter
	logger *slog.Logger
	apiKey string
}

func NewServer(modelRouter *router.ModelRouter, apiKey string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{router: modelRouter, apiKey: apiKey, logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /v1/models", s.handleModels)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)

	var handler http.Handler = mux
	handler = middleware.Auth(s.apiKey, s)(handler)
	handler = middleware.Logging(s.logger)(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.Recovery(s.logger, s)(handler)
	return handler
}

func (s *Server) WriteError(w http.ResponseWriter, err *compat.Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)
	_ = json.NewEncoder(w).Encode(compat.ErrorResponseFor(err))
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(compat.ModelListResponse{
		Object: "list",
		Data:   s.router.Models(),
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

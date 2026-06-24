package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/router"
)

type Server struct {
	router         *router.ModelRouter
	logger         *slog.Logger
	apiKey         string
	requestTimeout time.Duration
	streamTimeout  time.Duration
	rateLimiter    *middleware.RateLimiter
}

type Options struct {
	RequestTimeout time.Duration
	StreamTimeout  time.Duration
	RateLimiter    *middleware.RateLimiter
}

func NewServer(modelRouter *router.ModelRouter, apiKey string, logger *slog.Logger, options ...Options) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	opts := Options{
		RequestTimeout: 60 * time.Second,
		StreamTimeout:  10 * time.Minute,
	}
	if len(options) > 0 {
		opts = options[0]
		if opts.RequestTimeout == 0 {
			opts.RequestTimeout = 60 * time.Second
		}
		if opts.StreamTimeout == 0 {
			opts.StreamTimeout = 10 * time.Minute
		}
	}
	return &Server{
		router:         modelRouter,
		apiKey:         apiKey,
		logger:         logger,
		requestTimeout: opts.RequestTimeout,
		streamTimeout:  opts.StreamTimeout,
		rateLimiter:    opts.RateLimiter,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /v1/models", s.handleModels)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("POST /v1/embeddings", s.handleEmbeddings)

	var handler http.Handler = mux
	if s.rateLimiter != nil {
		handler = s.rateLimiter.Middleware(s)(handler)
	}
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

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err *compat.Error) {
	middleware.SetLogError(r.Context(), err.Type, err.Code)
	s.WriteError(w, err)
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

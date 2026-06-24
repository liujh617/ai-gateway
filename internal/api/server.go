package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/router"
	"open-ai-gateway/internal/version"
)

type Server struct {
	router         *router.ModelRouter
	logger         *slog.Logger
	apiKey         string
	requestTimeout time.Duration
	streamTimeout  time.Duration
	rateLimiter    *middleware.RateLimiter
	metrics        *middleware.Metrics
	maxBodyBytes   int64
}

type Options struct {
	RequestTimeout time.Duration
	StreamTimeout  time.Duration
	RateLimiter    *middleware.RateLimiter
	Metrics        *middleware.Metrics
	MaxBodyBytes   int64
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
	if opts.Metrics == nil {
		opts.Metrics = middleware.NewMetrics()
	}
	return &Server{
		router:         modelRouter,
		apiKey:         apiKey,
		logger:         logger,
		requestTimeout: opts.RequestTimeout,
		streamTimeout:  opts.StreamTimeout,
		rateLimiter:    opts.RateLimiter,
		metrics:        opts.Metrics,
		maxBodyBytes:   opts.MaxBodyBytes,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /version", s.handleVersion)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("GET /v1/models", s.handleModels)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("POST /v1/embeddings", s.handleEmbeddings)

	var handler http.Handler = mux
	if s.rateLimiter != nil {
		handler = s.rateLimiter.Middleware(s)(handler)
	}
	handler = middleware.Auth(s.apiKey, s)(handler)
	if s.metrics != nil {
		handler = s.metrics.Middleware(handler)
	}
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
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	modelCount := s.router.ModelCount()
	if modelCount == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		if r.Method == http.MethodHead {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "not_ready",
			"models": modelCount,
		})
		return
	}
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ready",
		"models": modelCount,
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	s.metrics.WritePrometheus(w)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(version.Current())
}

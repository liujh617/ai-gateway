package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/router"
	"open-ai-gateway/internal/routes"
	"open-ai-gateway/internal/version"
)

type Server struct {
	router         *router.ModelRouter
	logger         *slog.Logger
	credentials    []middleware.AuthCredential
	requestTimeout time.Duration
	streamTimeout  time.Duration
	rateLimiter    *middleware.RateLimiter
	metrics        *middleware.Metrics
	providerHealth *providerHealth
	clientModels   map[string]map[string]bool
	maxBodyBytes   int64
}

type Options struct {
	RequestTimeout        time.Duration
	StreamTimeout         time.Duration
	RateLimiter           *middleware.RateLimiter
	Metrics               *middleware.Metrics
	ProviderHealthOptions ProviderHealthOptions
	Credentials           []middleware.AuthCredential
	ClientModels          map[string][]string
	MaxBodyBytes          int64
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
	if opts.RateLimiter != nil {
		opts.RateLimiter.SetRejectionObserver(opts.Metrics)
	}
	credentials := append([]middleware.AuthCredential(nil), opts.Credentials...)
	if len(credentials) == 0 && apiKey != "" {
		credentials = []middleware.AuthCredential{{
			Client: "default",
			APIKey: apiKey,
		}}
	}
	providerHealth := newProviderHealth(opts.ProviderHealthOptions)
	for _, providerName := range modelRouter.ProviderNames() {
		providerHealth.Register(providerName)
		opts.Metrics.ObserveProviderHealth(providerName, true)
	}
	return &Server{
		router:         modelRouter,
		credentials:    credentials,
		logger:         logger,
		requestTimeout: opts.RequestTimeout,
		streamTimeout:  opts.StreamTimeout,
		rateLimiter:    opts.RateLimiter,
		metrics:        opts.Metrics,
		providerHealth: providerHealth,
		clientModels:   copyClientModels(opts.ClientModels),
		maxBodyBytes:   opts.MaxBodyBytes,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	handlers := s.routeHandlers()
	for _, route := range routes.All() {
		handler, ok := handlers[route.Path]
		if !ok {
			panic("missing handler for route " + route.Path)
		}
		mux.HandleFunc(route.RegistrationPattern(), handler)
	}
	mux.HandleFunc("/", s.handleNotFound)

	var handler http.Handler = mux
	handler = s.methodNotAllowed(handler)
	if s.rateLimiter != nil {
		handler = s.rateLimiter.Middleware(s)(handler)
	}
	handler = middleware.Auth(s.credentials, s)(handler)
	if s.metrics != nil {
		handler = s.metrics.Middleware(handler)
	}
	handler = middleware.Logging(s.logger)(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.Recovery(s.logger, s)(handler)
	handler = middleware.SecurityHeaders(handler)
	return handler
}

func (s *Server) routeHandlers() map[string]func(http.ResponseWriter, *http.Request) {
	return map[string]func(http.ResponseWriter, *http.Request){
		routes.HealthzPath:         s.handleHealthz,
		routes.ReadyzPath:          s.handleReadyz,
		routes.VersionPath:         s.handleVersion,
		routes.MetricsPath:         s.handleMetrics,
		routes.ModelsPath:          s.handleModels,
		routes.ChatCompletionsPath: s.handleChatCompletions,
		routes.EmbeddingsPath:      s.handleEmbeddings,
	}
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
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(compat.ModelListResponse{
		Object: "list",
		Data:   s.modelsForClient(middleware.ClientFromContext(r.Context())),
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

func (s *Server) modelAllowedForRequest(r *http.Request, model string) bool {
	client := middleware.ClientFromContext(r.Context())
	allowed, ok := s.clientModels[client]
	if !ok || len(allowed) == 0 {
		return true
	}
	return allowed[model]
}

func (s *Server) modelsForClient(client string) []compat.Model {
	models := s.router.Models()
	allowed, ok := s.clientModels[client]
	if !ok || len(allowed) == 0 {
		return models
	}
	filtered := models[:0]
	for _, model := range models {
		if allowed[model.ID] {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func copyClientModels(in map[string][]string) map[string]map[string]bool {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]map[string]bool, len(in))
	for client, models := range in {
		if client == "" || len(models) == 0 {
			continue
		}
		allowed := make(map[string]bool, len(models))
		for _, model := range models {
			if model != "" {
				allowed[model] = true
			}
		}
		if len(allowed) > 0 {
			out[client] = allowed
		}
	}
	return out
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
	s.syncProviderHealthMetrics()
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		return
	}
	s.metrics.WritePrometheus(w)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(version.Current())
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, r, compat.NotFound("route not found"))
}

func (s *Server) methodNotAllowed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, known := routes.MethodAllowed(r.URL.Path, r.Method)
		if !known || allowed {
			next.ServeHTTP(w, r)
			return
		}
		if allow, ok := routes.AllowHeader(r.URL.Path); ok {
			w.Header().Set("Allow", allow)
		}
		s.writeError(w, r, compat.MethodNotAllowed("method not allowed"))
	})
}

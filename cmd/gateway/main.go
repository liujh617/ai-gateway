package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"open-ai-gateway/internal/api"
	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/config"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/provider/fake"
	"open-ai-gateway/internal/provider/openai"
	"open-ai-gateway/internal/router"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	configPath := os.Getenv("GATEWAY_CONFIG")
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	logger = newLogger(cfg.Log)

	if shouldCheckConfig(os.Args, os.Getenv("GATEWAY_CHECK_CONFIG")) {
		if err := runConfigCheck(os.Stdout, configPath); err != nil {
			logger.Error("config check failed", "error", err)
			os.Exit(1)
		}
		return
	}

	modelRouter, err := buildRouter(cfg)
	if err != nil {
		logger.Error("failed to build model router", "error", err)
		os.Exit(1)
	}

	auditRecorder, err := buildAuditRecorder(cfg)
	if err != nil {
		logger.Error("failed to configure audit", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := auditRecorder.Close(); err != nil {
			logger.Debug("failed to close audit recorder", "error", err)
		}
	}()

	apiServer := api.NewServer(modelRouter, cfg.APIKey, logger, api.Options{
		RequestTimeout: cfg.RequestTimeout(),
		StreamTimeout:  cfg.StreamTimeout(),
		RateLimiter:    middleware.NewClientRateLimiter(cfg.RateLimit.RequestsPerMinute, gatewayClientRateLimits(cfg)),
		Credentials:    gatewayCredentials(cfg),
		ClientModels:   gatewayClientModels(cfg),
		ProviderHealthOptions: api.ProviderHealthOptions{
			FailureThreshold: cfg.ProviderHealth.FailureThreshold,
			Cooldown:         cfg.ProviderHealthCooldown(),
		},
		MaxBodyBytes: cfg.MaxRequestBodyBytes,
		Audit:        auditRecorder,
	})
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout(),
		ReadTimeout:       cfg.ReadTimeout(),
		WriteTimeout:      cfg.WriteTimeout(),
		IdleTimeout:       cfg.IdleTimeout(),
	}

	logger.Info("open-ai-gateway configured",
		"providers", cfg.ProviderNames(),
		"models", cfg.ModelNames(),
		"gateway_api_key_count", len(cfg.GatewayAPIClients()),
		"request_timeout_seconds", cfg.RequestTimeoutSeconds,
		"stream_timeout_seconds", cfg.StreamTimeoutSeconds,
		"read_header_timeout_seconds", cfg.ReadHeaderTimeoutSeconds,
		"read_timeout_seconds", cfg.ReadTimeoutSeconds,
		"write_timeout_seconds", cfg.WriteTimeoutSeconds,
		"idle_timeout_seconds", cfg.IdleTimeoutSeconds,
		"shutdown_timeout_seconds", cfg.ShutdownTimeoutSeconds,
		"max_request_body_bytes", cfg.MaxRequestBodyBytes,
		"log_format", cfg.Log.Format,
		"log_level", cfg.Log.Level,
		"audit_enabled", cfg.Audit.Enabled,
		"audit_path", cfg.Audit.Path,
		"rate_limit_requests_per_minute", cfg.RateLimit.RequestsPerMinute,
		"client_rate_limit_overrides", len(gatewayClientRateLimits(cfg)),
		"client_model_overrides", len(gatewayClientModels(cfg)),
		"provider_health_failure_threshold", cfg.ProviderHealth.FailureThreshold,
		"provider_health_cooldown_seconds", cfg.ProviderHealth.CooldownSeconds,
	)

	if err := serve(ctx, httpServer, cfg.ShutdownTimeout(), logger); err != nil {
		os.Exit(1)
	}
}

func shouldCheckConfig(args []string, env string) bool {
	if env == "1" || strings.EqualFold(env, "true") {
		return true
	}
	return len(args) > 1 && args[1] == "check-config"
}

func runConfigCheck(w io.Writer, path string) error {
	_, report, err := config.Check(path)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func buildAuditRecorder(cfg *config.Config) (audit.Recorder, error) {
	if !cfg.Audit.Enabled {
		return audit.NoopRecorder{}, nil
	}
	return audit.NewJSONLRecorder(cfg.Audit.Path)
}

func newLogger(cfg config.LogConfig) *slog.Logger {
	level := new(slog.LevelVar)
	level.Set(slogLevel(cfg.Level))
	options := &slog.HandlerOptions{Level: level}
	if cfg.Format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, options))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, options))
}

func slogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func serve(ctx context.Context, server *http.Server, shutdownTimeout time.Duration, logger *slog.Logger) error {
	errCh := make(chan error, 1)
	go func() {
		logger.Info("open-ai-gateway listening", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			return err
		}
		logger.Info("open-ai-gateway stopped")
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			logger.Info("open-ai-gateway stopped")
			return nil
		}
		logger.Error("server stopped", "error", err)
		return err
	}
}

func buildRouter(cfg *config.Config) (*router.ModelRouter, error) {
	providers := make(map[string]routerProvider, len(cfg.Providers))
	for name, providerConfig := range cfg.Providers {
		switch providerConfig.Type {
		case "fake":
			providers[name] = fake.New()
		case "openai-compatible":
			provider, err := openai.New(providerConfig.BaseURL, providerConfig.ResolvedAPIKey(), providerConfig.Timeout())
			if err != nil {
				return nil, fmt.Errorf("provider %q: %w", name, err)
			}
			providers[name] = provider
		default:
			return nil, fmt.Errorf("provider %q has unsupported type %q", name, providerConfig.Type)
		}
	}

	routes := make([]router.ModelRoute, 0, len(cfg.Models))
	for externalModel, modelConfig := range cfg.Models {
		provider, ok := providers[modelConfig.Provider]
		if !ok {
			return nil, fmt.Errorf("model %q references unknown provider %q", externalModel, modelConfig.Provider)
		}
		upstreamModel := modelConfig.UpstreamModel
		if upstreamModel == "" {
			upstreamModel = externalModel
		}
		fallbacks, err := fallbackRoutes(externalModel, modelConfig.Fallbacks, providers)
		if err != nil {
			return nil, err
		}
		routes = append(routes, router.ModelRoute{
			ExternalModel: externalModel,
			UpstreamModel: upstreamModel,
			ProviderName:  modelConfig.Provider,
			Capabilities:  capabilities(modelConfig.Capabilities),
			Provider:      provider,
			Pricing:       pricing(modelConfig.Pricing),
			Fallbacks:     fallbacks,
		})
	}
	return router.NewModelRouter(routes), nil
}

type routerProvider = provider.Provider

func capabilities(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func fallbackRoutes(externalModel string, fallbacks []config.ModelFallbackConfig, providers map[string]routerProvider) ([]router.ProviderRoute, error) {
	routes := make([]router.ProviderRoute, 0, len(fallbacks))
	for _, fallback := range fallbacks {
		provider, ok := providers[fallback.Provider]
		if !ok {
			return nil, fmt.Errorf("model %q fallback references unknown provider %q", externalModel, fallback.Provider)
		}
		upstreamModel := fallback.UpstreamModel
		if upstreamModel == "" {
			upstreamModel = externalModel
		}
		routes = append(routes, router.ProviderRoute{
			UpstreamModel: upstreamModel,
			ProviderName:  fallback.Provider,
			Provider:      provider,
			Pricing:       pricing(fallback.Pricing),
		})
	}
	return routes, nil
}

func gatewayCredentials(cfg *config.Config) []middleware.AuthCredential {
	clients := cfg.GatewayAPIClients()
	credentials := make([]middleware.AuthCredential, 0, len(clients))
	for _, client := range clients {
		credentials = append(credentials, middleware.AuthCredential{
			Client: client.Name,
			APIKey: client.APIKey,
		})
	}
	return credentials
}

func gatewayClientRateLimits(cfg *config.Config) map[string]int {
	clients := cfg.GatewayAPIClients()
	limits := make(map[string]int)
	for _, client := range clients {
		if client.RateLimit.RequestsPerMinute != nil {
			limits[client.Name] = *client.RateLimit.RequestsPerMinute
		}
	}
	return limits
}

func gatewayClientModels(cfg *config.Config) map[string][]string {
	clients := cfg.GatewayAPIClients()
	models := make(map[string][]string)
	for _, client := range clients {
		if len(client.Models) > 0 {
			models[client.Name] = append([]string(nil), client.Models...)
		}
	}
	return models
}

func pricing(value config.PricingConfig) router.TokenPricing {
	return router.TokenPricing{
		PromptUSDPer1MTokens:     value.PromptUSDPer1MTokens,
		CompletionUSDPer1MTokens: value.CompletionUSDPer1MTokens,
	}
}

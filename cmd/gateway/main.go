package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"open-ai-gateway/internal/api"
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

	cfg, err := config.Load(os.Getenv("GATEWAY_CONFIG"))
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	modelRouter, err := buildRouter(cfg)
	if err != nil {
		logger.Error("failed to build model router", "error", err)
		os.Exit(1)
	}

	apiServer := api.NewServer(modelRouter, cfg.APIKey, logger, api.Options{
		RequestTimeout: cfg.RequestTimeout(),
		StreamTimeout:  cfg.StreamTimeout(),
		RateLimiter:    middleware.NewRateLimiter(cfg.RateLimit.RequestsPerMinute),
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
		"request_timeout_seconds", cfg.RequestTimeoutSeconds,
		"stream_timeout_seconds", cfg.StreamTimeoutSeconds,
		"read_header_timeout_seconds", cfg.ReadHeaderTimeoutSeconds,
		"read_timeout_seconds", cfg.ReadTimeoutSeconds,
		"write_timeout_seconds", cfg.WriteTimeoutSeconds,
		"idle_timeout_seconds", cfg.IdleTimeoutSeconds,
		"shutdown_timeout_seconds", cfg.ShutdownTimeoutSeconds,
		"rate_limit_requests_per_minute", cfg.RateLimit.RequestsPerMinute,
	)

	if err := serve(ctx, httpServer, cfg.ShutdownTimeout(), logger); err != nil {
		os.Exit(1)
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
		routes = append(routes, router.ModelRoute{
			ExternalModel: externalModel,
			UpstreamModel: upstreamModel,
			ProviderName:  modelConfig.Provider,
			Capabilities:  capabilities(modelConfig.Capabilities),
			Provider:      provider,
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

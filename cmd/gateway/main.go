package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"open-ai-gateway/internal/api"
	"open-ai-gateway/internal/config"
	"open-ai-gateway/internal/provider"
	"open-ai-gateway/internal/provider/fake"
	"open-ai-gateway/internal/provider/openai"
	"open-ai-gateway/internal/router"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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

	server := api.NewServer(modelRouter, cfg.APIKey, logger)

	logger.Info("open-ai-gateway listening", "addr", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, server.Handler()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
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
			Provider:      provider,
		})
	}
	return router.NewModelRouter(routes), nil
}

type routerProvider = provider.Provider

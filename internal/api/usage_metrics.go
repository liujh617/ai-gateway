package api

import (
	"context"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/router"
)

func (s *Server) observeUsage(path, externalModel, providerName, client string, usage *compat.Usage, pricing router.TokenPricing) {
	if usage == nil || s.metrics == nil {
		return
	}
	if client == "" {
		client = "unconfigured"
	}
	s.metrics.ObserveTokens(path, externalModel, providerName, "prompt", client, usage.PromptTokens)
	s.metrics.ObserveTokens(path, externalModel, providerName, "completion", client, usage.CompletionTokens)
	s.metrics.ObserveTokens(path, externalModel, providerName, "total", client, usage.TotalTokens)
	s.observeCost(path, externalModel, providerName, usage, pricing, client)
}

func (s *Server) observeCost(path, externalModel, providerName string, usage *compat.Usage, pricing router.TokenPricing, client string) {
	promptCost := tokenCostUSD(usage.PromptTokens, pricing.PromptUSDPer1MTokens)
	completionCost := tokenCostUSD(usage.CompletionTokens, pricing.CompletionUSDPer1MTokens)
	s.metrics.ObserveTokenCostUSD(path, externalModel, providerName, "prompt", client, promptCost)
	s.metrics.ObserveTokenCostUSD(path, externalModel, providerName, "completion", client, completionCost)
	s.metrics.ObserveTokenCostUSD(path, externalModel, providerName, "total", client, promptCost+completionCost)
}

func tokenCostUSD(tokens int, usdPer1MTokens float64) float64 {
	if tokens <= 0 || usdPer1MTokens <= 0 {
		return 0
	}
	return float64(tokens) * usdPer1MTokens / 1_000_000
}

func clientFromContext(ctx context.Context) string {
	return middleware.ClientFromContext(ctx)
}

func (s *Server) observeProviderFallback(ctx context.Context, path, externalModel, fromProvider, toProvider string) {
	if s.metrics == nil {
		return
	}
	client := clientFromContext(ctx)
	if client == "" {
		client = "unconfigured"
	}
	s.metrics.ObserveProviderFallback(path, externalModel, fromProvider, toProvider, client)
}

func (s *Server) observeProviderCircuitOpen(ctx context.Context, path, externalModel, providerName string) {
	if s.metrics == nil {
		return
	}
	client := clientFromContext(ctx)
	if client == "" {
		client = "unconfigured"
	}
	s.metrics.ObserveProviderCircuitOpen(path, externalModel, providerName, client)
}

func (s *Server) observeProviderHealth(providerName string) {
	if s.metrics == nil || s.providerHealth == nil {
		return
	}
	for _, item := range s.providerHealth.Snapshot() {
		if item.Provider == providerName {
			s.metrics.ObserveProviderHealth(item.Provider, item.Healthy)
			return
		}
	}
}

func (s *Server) syncProviderHealthMetrics() {
	if s.metrics == nil || s.providerHealth == nil {
		return
	}
	for _, item := range s.providerHealth.Snapshot() {
		s.metrics.ObserveProviderHealth(item.Provider, item.Healthy)
	}
}

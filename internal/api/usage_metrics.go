package api

import "open-ai-gateway/internal/compat"

func (s *Server) observeUsage(path, externalModel, providerName string, usage *compat.Usage) {
	if usage == nil || s.metrics == nil {
		return
	}
	s.metrics.ObserveTokens(path, externalModel, providerName, "prompt", usage.PromptTokens)
	s.metrics.ObserveTokens(path, externalModel, providerName, "completion", usage.CompletionTokens)
	s.metrics.ObserveTokens(path, externalModel, providerName, "total", usage.TotalTokens)
}

func (s *Server) observeProviderFallback(path, externalModel, fromProvider, toProvider string) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveProviderFallback(path, externalModel, fromProvider, toProvider)
}

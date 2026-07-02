package api

import (
	"context"
	"encoding/json"
	"net/http"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/router"
	"open-ai-gateway/internal/routes"
)

func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if err := requireJSONContentType(r); err != nil {
		s.writeError(w, r, err)
		return
	}
	var req compat.EmbeddingRequest
	if err := decodeJSONBody(s.requestBody(w, r), &req); err != nil {
		s.writeError(w, r, decodeError(err))
		return
	}
	middleware.SetLogStream(r.Context(), false)
	if err := req.Validate(); err != nil {
		s.writeError(w, r, err)
		return
	}

	route, resolveErr := s.router.ResolveFor(req.Model, "embeddings")
	if resolveErr != nil {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeError(w, r, resolveErr)
		return
	}

	externalModel := req.Model

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()

	resp, err := s.createEmbeddingWithFallback(ctx, r, route, externalModel, req)
	if err != nil {
		s.writeError(w, r, providerError(err))
		return
	}
	resp.Model = externalModel

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) createEmbeddingWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
	var lastErr error
	var skippedFrom string
	attempts := route.Attempts()
	for index, attempt := range attempts {
		if !s.providerHealth.Healthy(attempt.ProviderName) {
			s.observeProviderHealth(attempt.ProviderName)
			if skippedFrom == "" {
				skippedFrom = attempt.ProviderName
			}
			s.logger.Warn("embedding provider circuit open; trying fallback", "provider", attempt.ProviderName)
			continue
		}
		if skippedFrom != "" {
			s.observeProviderFallback(routes.EmbeddingsPath, externalModel, skippedFrom, attempt.ProviderName)
			skippedFrom = ""
		}
		attemptReq := req
		attemptReq.Model = attempt.UpstreamModel
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		resp, err := attempt.Provider.CreateEmbedding(ctx, attemptReq)
		if err == nil {
			s.providerHealth.MarkSuccess(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
			s.observeUsage(routes.EmbeddingsPath, externalModel, attempt.ProviderName, clientFromContext(r.Context()), resp.Usage, attempt.Pricing)
			return resp, nil
		}
		lastErr = err
		if canFallbackProviderError(err) {
			s.providerHealth.MarkFailure(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
		}
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return nil, err
		}
		if nextProviderName := s.nextHealthyProviderName(attempts[index+1:]); nextProviderName != "" {
			s.observeProviderFallback(routes.EmbeddingsPath, externalModel, attempt.ProviderName, nextProviderName)
		}
		s.logger.Warn("embedding provider failed; trying fallback", "provider", attempt.ProviderName, "error", err)
	}
	if skippedFrom != "" {
		return nil, providerUnavailableError()
	}
	return nil, lastErr
}

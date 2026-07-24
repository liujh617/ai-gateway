package api

import (
	"context"
	"encoding/json"
	"net/http"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/provider"
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
		s.writeAuditedError(w, r, routes.EmbeddingsPath, req.Model, err)
		return
	}
	if !s.modelAllowedForRequest(r, req.Model) {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeAuditedError(w, r, routes.EmbeddingsPath, req.Model, compat.ModelNotFound(req.Model))
		return
	}

	route, resolveErr := s.router.ResolveFor(req.Model, "embeddings")
	if resolveErr != nil {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeAuditedError(w, r, routes.EmbeddingsPath, req.Model, resolveErr)
		return
	}

	externalModel := req.Model
	requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.EmbeddingsPath, externalModel)
	requestEvent.Body = rawBody(req)
	s.audit.Record(r.Context(), requestEvent)

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()

	resp, providerName, upstreamModel, err := s.createEmbeddingWithFallback(ctx, r, route, externalModel, req)
	if err != nil {
		s.writeAuditedError(w, r, routes.EmbeddingsPath, externalModel, providerError(err))
		return
	}
	resp.Model = externalModel
	responseEvent := s.auditBaseEvent(r, audit.EventResponse, routes.EmbeddingsPath, externalModel)
	responseEvent.Provider = providerName
	responseEvent.UpstreamModel = upstreamModel
	responseEvent.Status = http.StatusOK
	responseEvent.Body = rawBody(resp)
	s.audit.Record(r.Context(), responseEvent)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) createEmbeddingWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, string, string, error) {
	return executeWithFallback(s, ctx, r, routes.EmbeddingsPath, externalModel, route, req,
		func(ctx context.Context, p provider.Provider, req compat.EmbeddingRequest) (*compat.EmbeddingResponse, error) {
			return p.CreateEmbedding(ctx, req)
		},
		func(req compat.EmbeddingRequest, upstreamModel string) compat.EmbeddingRequest {
			req.Model = upstreamModel
			return req
		},
		func(resp *compat.EmbeddingResponse, fa fallbackAttempt) {
			s.observeUsage(routes.EmbeddingsPath, externalModel, fa.ProviderName, clientFromContext(r.Context()), resp.Usage, fa.Pricing)
		},
	)
}

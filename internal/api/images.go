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

func (s *Server) handleImageGenerations(w http.ResponseWriter, r *http.Request) {
	if err := requireJSONContentType(r); err != nil {
		s.writeError(w, r, err)
		return
	}
	var req compat.ImageGenerationRequest
	if err := decodeJSONBody(s.requestBody(w, r), &req); err != nil {
		s.writeError(w, r, decodeError(err))
		return
	}
	if validationErr := req.Validate(); validationErr != nil {
		s.writeAuditedError(w, r, routes.ImageGenerationsPath, req.Model, validationErr)
		return
	}
	if !s.modelAllowedForRequest(r, req.Model) {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeAuditedError(w, r, routes.ImageGenerationsPath, req.Model, compat.ModelNotFound(req.Model))
		return
	}
	route, resolveErr := s.router.ResolveFor(req.Model, "images")
	if resolveErr != nil {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeAuditedError(w, r, routes.ImageGenerationsPath, req.Model, resolveErr)
		return
	}
	externalModel := req.Model
	requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.ImageGenerationsPath, externalModel)
	requestEvent.Body = rawBody(req)
	s.audit.Record(r.Context(), requestEvent)

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()
	resp, providerName, upstreamModel, err := s.createImageWithFallback(ctx, r, route, externalModel, req)
	if err != nil {
		s.writeAuditedError(w, r, routes.ImageGenerationsPath, externalModel, providerError(err))
		return
	}
	responseEvent := s.auditBaseEvent(r, audit.EventResponse, routes.ImageGenerationsPath, externalModel)
	responseEvent.Provider = providerName
	responseEvent.UpstreamModel = upstreamModel
	responseEvent.Status = http.StatusOK
	responseEvent.Body = rawBody(resp)
	s.audit.Record(r.Context(), responseEvent)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) createImageWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req compat.ImageGenerationRequest) (*compat.ImageGenerationResponse, string, string, error) {
	return executeWithFallback(s, ctx, r, routes.ImageGenerationsPath, externalModel, route, req,
		func(ctx context.Context, p provider.Provider, req compat.ImageGenerationRequest) (*compat.ImageGenerationResponse, error) {
			return p.CreateImage(ctx, req)
		},
		func(req compat.ImageGenerationRequest, upstreamModel string) compat.ImageGenerationRequest {
			req.Model = upstreamModel
			return req
		},
		nil,
	)
}

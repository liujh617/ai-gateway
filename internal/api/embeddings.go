package api

import (
	"context"
	"encoding/json"
	"net/http"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
)

func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	var req compat.EmbeddingRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		s.writeError(w, r, compat.InvalidRequest("invalid JSON request body", "body"))
		return
	}
	middleware.SetLogStream(r.Context(), false)
	if err := req.Validate(); err != nil {
		s.writeError(w, r, err)
		return
	}

	route, resolveErr := s.router.Resolve(req.Model)
	if resolveErr != nil {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeError(w, r, resolveErr)
		return
	}

	externalModel := req.Model
	req.Model = route.UpstreamModel
	middleware.SetLogRoute(r.Context(), externalModel, route.ProviderName, route.UpstreamModel)

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()

	resp, err := route.Provider.CreateEmbedding(ctx, req)
	if err != nil {
		s.writeError(w, r, providerError(err))
		return
	}
	resp.Model = externalModel

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

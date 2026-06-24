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
	decoder := json.NewDecoder(s.requestBody(w, r))
	if err := decoder.Decode(&req); err != nil {
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

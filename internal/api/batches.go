package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/routes"
)

func (s *Server) handleBatches(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateBatch(w, r)
	case http.MethodGet:
		s.handleListBatches(w, r)
	default:
		s.writeError(w, r, compat.ServerError(http.StatusMethodNotAllowed, "method not allowed"))
	}
}

func (s *Server) handleCreateBatch(w http.ResponseWriter, r *http.Request) {
	if err := requireJSONContentType(r); err != nil {
		s.writeError(w, r, err)
		return
	}
	var req compat.BatchRequest
	if err := decodeJSONBody(s.requestBody(w, r), &req); err != nil {
		s.writeError(w, r, decodeError(err))
		return
	}
	if err := req.Validate(); err != nil {
		s.writeAuditedError(w, r, routes.BatchesPath, "", err)
		return
	}
	route, ok := s.resolveCapabilityRoute(w, r, "batches")
	if !ok {
		return
	}
	requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.BatchesPath, "")
	requestEvent.Body = rawBody(req)
	s.audit.Record(r.Context(), requestEvent)

	batch, err := route.Provider.CreateBatch(r.Context(), req)
	if err != nil {
		s.writeAuditedError(w, r, routes.BatchesPath, "", providerError(err))
		return
	}
	responseEvent := s.auditBaseEvent(r, audit.EventResponse, routes.BatchesPath, "")
	responseEvent.Provider = route.ProviderName
	responseEvent.Status = http.StatusOK
	responseEvent.Body = rawBody(batch)
	s.audit.Record(r.Context(), responseEvent)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(batch)
}

func (s *Server) handleListBatches(w http.ResponseWriter, r *http.Request) {
	after := r.URL.Query().Get("after")
	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	route, ok := s.resolveCapabilityRoute(w, r, "batches")
	if !ok {
		return
	}
	list, err := route.Provider.ListBatches(r.Context(), after, limit)
	if err != nil {
		s.writeAuditedError(w, r, routes.BatchesPath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (s *Server) handleRetrieveBatch(w http.ResponseWriter, r *http.Request) {
	batchID := strings.TrimPrefix(r.URL.Path, routes.BatchesPath+"/")
	batchID = strings.TrimSuffix(batchID, "/cancel")
	if batchID == "" {
		s.writeError(w, r, compat.InvalidRequest("missing batch id", "batch_id"))
		return
	}
	route, ok := s.resolveCapabilityRoute(w, r, "batches")
	if !ok {
		return
	}
	batch, err := route.Provider.RetrieveBatch(r.Context(), batchID)
	if err != nil {
		s.writeAuditedError(w, r, routes.BatchesRetrievePath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(batch)
}

func (s *Server) handleCancelBatch(w http.ResponseWriter, r *http.Request) {
	batchID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, routes.BatchesPath+"/"), "/cancel")
	if batchID == "" {
		s.writeError(w, r, compat.InvalidRequest("missing batch id", "batch_id"))
		return
	}
	route, ok := s.resolveCapabilityRoute(w, r, "batches")
	if !ok {
		return
	}
	batch, err := route.Provider.CancelBatch(r.Context(), batchID)
	if err != nil {
		s.writeAuditedError(w, r, routes.BatchesCancelPath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(batch)
}

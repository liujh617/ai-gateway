package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/routes"
)

func (s *Server) handleFineTuningJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateFineTuningJob(w, r)
	case http.MethodGet:
		s.handleListFineTuningJobs(w, r)
	default:
		s.writeError(w, r, compat.ServerError(http.StatusMethodNotAllowed, "method not allowed"))
	}
}

func (s *Server) handleCreateFineTuningJob(w http.ResponseWriter, r *http.Request) {
	if err := requireJSONContentType(r); err != nil {
		s.writeError(w, r, err)
		return
	}
	var req compat.FineTuningJobRequest
	if err := decodeJSONBody(s.requestBody(w, r), &req); err != nil {
		s.writeError(w, r, decodeError(err))
		return
	}
	if err := req.Validate(); err != nil {
		s.writeAuditedError(w, r, routes.FineTuningJobsPath, req.Model, err)
		return
	}
	route, ok := s.resolveCapabilityRoute(w, r, "fine_tuning")
	if !ok {
		return
	}
	job, err := route.Provider.CreateFineTuningJob(r.Context(), req)
	if err != nil {
		s.writeAuditedError(w, r, routes.FineTuningJobsPath, req.Model, providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}

func (s *Server) handleListFineTuningJobs(w http.ResponseWriter, r *http.Request) {
	after := r.URL.Query().Get("after")
	limit := parseLimit(r.URL.Query().Get("limit"))
	route, ok := s.resolveCapabilityRoute(w, r, "fine_tuning")
	if !ok {
		return
	}
	list, err := route.Provider.ListFineTuningJobs(r.Context(), after, limit)
	if err != nil {
		s.writeAuditedError(w, r, routes.FineTuningJobsPath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (s *Server) handleRetrieveFineTuningJob(w http.ResponseWriter, r *http.Request) {
	jobID := fineTuningJobID(r.URL.Path, routes.FineTuningJobsPath)
	if jobID == "" {
		s.writeError(w, r, compat.InvalidRequest("missing fine_tuning_job id", "job_id"))
		return
	}
	route, ok := s.resolveCapabilityRoute(w, r, "fine_tuning")
	if !ok {
		return
	}
	job, err := route.Provider.RetrieveFineTuningJob(r.Context(), jobID)
	if err != nil {
		s.writeAuditedError(w, r, routes.FineTuningJobRetrievePath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}

func (s *Server) handleFineTuningJobEvents(w http.ResponseWriter, r *http.Request) {
	jobID := fineTuningJobID(r.URL.Path, routes.FineTuningJobsPath)
	if jobID == "" {
		s.writeError(w, r, compat.InvalidRequest("missing fine_tuning_job id", "job_id"))
		return
	}
	after := r.URL.Query().Get("after")
	limit := parseLimit(r.URL.Query().Get("limit"))
	route, ok := s.resolveCapabilityRoute(w, r, "fine_tuning")
	if !ok {
		return
	}
	events, err := route.Provider.ListFineTuningJobEvents(r.Context(), jobID, after, limit)
	if err != nil {
		s.writeAuditedError(w, r, routes.FineTuningJobEventsPath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(events)
}

func (s *Server) handleFineTuningJobCheckpoints(w http.ResponseWriter, r *http.Request) {
	jobID := fineTuningJobID(r.URL.Path, routes.FineTuningJobsPath)
	if jobID == "" {
		s.writeError(w, r, compat.InvalidRequest("missing fine_tuning_job id", "job_id"))
		return
	}
	after := r.URL.Query().Get("after")
	limit := parseLimit(r.URL.Query().Get("limit"))
	route, ok := s.resolveCapabilityRoute(w, r, "fine_tuning")
	if !ok {
		return
	}
	checkpoints, err := route.Provider.ListFineTuningJobCheckpoints(r.Context(), jobID, after, limit)
	if err != nil {
		s.writeAuditedError(w, r, routes.FineTuningJobCheckpointsPath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(checkpoints)
}

func (s *Server) handleCancelFineTuningJob(w http.ResponseWriter, r *http.Request) {
	jobID := fineTuningJobID(r.URL.Path, routes.FineTuningJobsPath)
	if jobID == "" {
		s.writeError(w, r, compat.InvalidRequest("missing fine_tuning_job id", "job_id"))
		return
	}
	route, ok := s.resolveCapabilityRoute(w, r, "fine_tuning")
	if !ok {
		return
	}
	job, err := route.Provider.CancelFineTuningJob(r.Context(), jobID)
	if err != nil {
		s.writeAuditedError(w, r, routes.FineTuningJobCancelPath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}

func fineTuningJobID(path, prefix string) string {
	suffix := strings.TrimPrefix(path, prefix+"/")
	if suffix == "" || suffix == path {
		return ""
	}
	for _, sub := range []string{"/events", "/cancel", "/checkpoints"} {
		suffix = strings.TrimSuffix(suffix, sub)
	}
	if strings.Contains(suffix, "/") {
		return ""
	}
	return suffix
}

func parseLimit(s string) int {
	if s == "" {
		return 20
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return 20
	}
	return v
}

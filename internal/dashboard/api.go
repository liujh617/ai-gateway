package dashboard

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// API represents the dashboard API server
type API struct {
	config    *Config
	logReader *LogReader
	stats     *StatsCollector
}

// Config holds dashboard configuration
type Config struct {
	AuditPath       string
	AlertsPath      string
	EncryptionKey   []byte
	MaxLogLines     int
	EnableDecryption bool
}

// NewAPI creates a new dashboard API instance
func NewAPI(cfg *Config) (*API, error) {
	logReader, err := NewLogReader(cfg.AuditPath, cfg.EncryptionKey, cfg.EnableDecryption)
	if err != nil {
		return nil, err
	}

	stats := NewStatsCollector(logReader)

	return &API{
		config:    cfg,
		logReader: logReader,
		stats:     stats,
	}, nil
}

// Router returns the chi router for the dashboard API
func (a *API) Router() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	// CORS headers (for development)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Status and Health
		r.Get("/status", a.handleStatus)
		r.Get("/stats", a.handleStats)

		// Audit Logs
		r.Get("/audit/logs", a.handleAuditLogs)
		r.Get("/audit/logs/{id}", a.handleAuditLogDetail)

		// PII Detection
		r.Get("/pii/violations", a.handlePIIViolations)
		r.Get("/pii/stats", a.handlePIIStats)

		// Content Safety
		r.Get("/content-safety/violations", a.handleContentSafetyViolations)
		r.Get("/content-safety/stats", a.handleContentSafetyStats)

		// Alerts
		r.Get("/alerts", a.handleAlerts)
		r.Get("/alerts/stats", a.handleAlertsStats)
	})

	// Serve static files (will be implemented in integration phase)
	// r.Get("/dashboard/*", serveStaticFiles)

	return r
}

// handleStatus returns gateway status
func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "running",
		"version":   "1.4.0",
		"timestamp": getCurrentTimestamp(),
		"features": map[string]bool{
			"audit_enabled":        true,
			"pii_detection":        true,
			"content_safety":       true,
			"alerting":             true,
		},
	}

	writeJSON(w, http.StatusOK, status)
}

// handleStats returns summary statistics
func (a *API) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := a.stats.GetSummary(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// handleAuditLogs returns audit logs with filters
func (a *API) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	filter := ParseAuditLogFilter(r)

	logs, total, err := a.logReader.ListLogs(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	response := map[string]interface{}{
		"logs":  logs,
		"total": total,
		"page":  filter.Page,
		"size":  filter.Size,
	}

	writeJSON(w, http.StatusOK, response)
}

// handleAuditLogDetail returns a single audit log detail
func (a *API) handleAuditLogDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := chi.URLParam(r, "id")

	log, err := a.logReader.GetLogDetail(ctx, requestID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "audit log not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get audit log")
		return
	}

	writeJSON(w, http.StatusOK, log)
}

// handlePIIViolations returns PII violations list
func (a *API) handlePIIViolations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := ParsePIIViolationFilter(r)

	violations, total, err := a.stats.GetPIIViolations(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get PII violations")
		return
	}

	response := map[string]interface{}{
		"violations": violations,
		"total":      total,
		"page":       filter.Page,
		"size":       filter.Size,
	}

	writeJSON(w, http.StatusOK, response)
}

// handlePIIStats returns PII statistics
func (a *API) handlePIIStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := a.stats.GetPIIStats(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get PII stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// handleContentSafetyViolations returns content safety violations list
func (a *API) handleContentSafetyViolations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := ParseContentSafetyViolationFilter(r)

	violations, total, err := a.stats.GetContentSafetyViolations(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get content safety violations")
		return
	}

	response := map[string]interface{}{
		"violations": violations,
		"total":      total,
		"page":       filter.Page,
		"size":       filter.Size,
	}

	writeJSON(w, http.StatusOK, response)
}

// handleContentSafetyStats returns content safety statistics
func (a *API) handleContentSafetyStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := a.stats.GetContentSafetyStats(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get content safety stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// handleAlerts returns alerts list
func (a *API) handleAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := ParseAlertFilter(r)

	alerts, total, err := a.stats.GetAlerts(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get alerts")
		return
	}

	response := map[string]interface{}{
		"alerts": alerts,
		"total":  total,
		"page":   filter.Page,
		"size":   filter.Size,
	}

	writeJSON(w, http.StatusOK, response)
}

// handleAlertsStats returns alert statistics
func (a *API) handleAlertsStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := a.stats.GetAlertStats(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get alert stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
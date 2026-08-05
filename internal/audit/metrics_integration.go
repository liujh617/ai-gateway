package audit

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Audit metrics
	auditLogsTotal *prometheus.CounterVec
	auditLogSize   *prometheus.GaugeVec
	auditErrorsTotal *prometheus.CounterVec

	// PII metrics
	piiDetectedTotal *prometheus.CounterVec
	piiBlockedTotal  *prometheus.CounterVec

	// Content safety metrics
	contentSafetyViolationsTotal *prometheus.CounterVec
	contentSafetyBlockedTotal    *prometheus.CounterVec

	// Encryption metrics
	encryptionOpsTotal *prometheus.CounterVec
)

// init registers all audit-related Prometheus metrics
func init() {
	// Audit metrics
	auditLogsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_audit_logs_total",
			Help: "Total number of audit log entries written",
		},
		[]string{"provider", "model", "event_type"},
	)

	auditLogSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ai_gateway_audit_log_size_bytes",
			Help: "Current size of audit log file in bytes",
		},
		[]string{"file"},
	)

	auditErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_audit_errors_total",
			Help: "Total number of audit errors",
		},
		[]string{"error_type"},
	)

	// PII metrics
	piiDetectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_pii_detected_total",
			Help: "Total number of PII detections",
		},
		[]string{"provider", "model", "pii_type", "action"},
	)

	piiBlockedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_pii_blocked_total",
			Help: "Total number of requests blocked by PII detection",
		},
		[]string{"provider", "model"},
	)

	// Content safety metrics
	contentSafetyViolationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_content_safety_violations_total",
			Help: "Total number of content safety violations detected",
		},
		[]string{"provider", "model", "category", "action"},
	)

	contentSafetyBlockedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_content_safety_blocked_total",
			Help: "Total number of requests blocked by content safety detection",
		},
		[]string{"provider", "model"},
	)

	// Encryption metrics
	encryptionOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_encryption_operations_total",
			Help: "Total number of encryption operations",
		},
		[]string{"operation", "status"},
	)

	// Register all metrics
	prometheus.MustRegister(
		auditLogsTotal,
		auditLogSize,
		auditErrorsTotal,
		piiDetectedTotal,
		piiBlockedTotal,
		contentSafetyViolationsTotal,
		contentSafetyBlockedTotal,
		encryptionOpsTotal,
	)
}

// RecordAuditLog increments audit log counter
func RecordAuditLog(provider, model, eventType string) {
	auditLogsTotal.WithLabelValues(provider, model, eventType).Inc()
}

// RecordAuditLogSize updates audit log file size gauge
func RecordAuditLogSize(file string, sizeBytes float64) {
	auditLogSize.WithLabelValues(file).Set(sizeBytes)
}

// RecordAuditError increments audit error counter
func RecordAuditError(errorType string) {
	auditErrorsTotal.WithLabelValues(errorType).Inc()
}

// RecordPIIDetected increments PII detection counter
func RecordPIIDetected(provider, model, piiType, action string) {
	piiDetectedTotal.WithLabelValues(provider, model, piiType, action).Inc()
}

// RecordPIIBlocked increments PII blocked counter
func RecordPIIBlocked(provider, model string) {
	piiBlockedTotal.WithLabelValues(provider, model).Inc()
}

// RecordContentSafetyViolation increments content safety violation counter
func RecordContentSafetyViolation(provider, model, category, action string) {
	contentSafetyViolationsTotal.WithLabelValues(provider, model, category, action).Inc()
}

// RecordContentSafetyBlocked increments content safety blocked counter
func RecordContentSafetyBlocked(provider, model string) {
	contentSafetyBlockedTotal.WithLabelValues(provider, model).Inc()
}

// RecordEncryptionOp increments encryption operation counter
func RecordEncryptionOp(operation, status string) {
	encryptionOpsTotal.WithLabelValues(operation, status).Inc()
}

// InstrumentPIIDetection wraps PII detection with metrics recording
func InstrumentPIIDetection(ctx context.Context, provider, model string, result *PIIDetectionResult, action PIIDetectionAction) {
	if result == nil || !result.HasPII {
		return
	}

	for piiType, count := range result.TypeCounts {
		for i := 0; i < count; i++ {
			RecordPIIDetected(provider, model, string(piiType), string(action))
		}
	}

	if action == PIIActionReject {
		RecordPIIBlocked(provider, model)
	}
}

// InstrumentContentSafetyDetection wraps content safety detection with metrics recording
func InstrumentContentSafetyDetection(ctx context.Context, provider, model string, result *ContentSafetyResult, action ContentSafetyAction) {
	if result == nil || !result.HasViolation {
		return
	}

	for category, violations := range result.CategoryViolations {
		for range violations {
			RecordContentSafetyViolation(provider, model, string(category), string(action))
		}
	}

	if action == ContentSafetyActionReject {
		RecordContentSafetyBlocked(provider, model)
	}
}
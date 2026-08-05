package alert

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Alert metrics
	alertsCreatedTotal *prometheus.CounterVec
	alertsSentTotal    *prometheus.CounterVec
	alertsFailedTotal  *prometheus.CounterVec
	alertsSuppressed   *prometheus.CounterVec
	alertDuration      *prometheus.HistogramVec
)

// init registers all alert-related Prometheus metrics
func init() {
	alertsCreatedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_alerts_created_total",
			Help: "Total number of alerts created",
		},
		[]string{"source", "severity"},
	)

	alertsSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_alerts_sent_total",
			Help: "Total number of alerts successfully sent",
		},
		[]string{"channel", "severity"},
	)

	alertsFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_alerts_failed_total",
			Help: "Total number of alerts that failed to send",
		},
		[]string{"channel", "severity", "error_type"},
	)

	alertsSuppressed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_gateway_alerts_suppressed_total",
			Help: "Total number of alerts suppressed by rate limiter or cooldown",
		},
		[]string{"reason"},
	)

	alertDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "ai_gateway_alert_delivery_duration_seconds",
			Help: "Time taken to deliver an alert",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"channel"},
	)

	prometheus.MustRegister(
		alertsCreatedTotal,
		alertsSentTotal,
		alertsFailedTotal,
		alertsSuppressed,
		alertDuration,
	)
}

// RecordAlertCreated increments alert created counter
func RecordAlertCreated(source, severity string) {
	alertsCreatedTotal.WithLabelValues(source, severity).Inc()
}

// RecordAlertSent increments alert sent counter
func RecordAlertSent(channel, severity string) {
	alertsSentTotal.WithLabelValues(channel, severity).Inc()
}

// RecordAlertFailed increments alert failed counter
func RecordAlertFailed(channel, severity, errorType string) {
	alertsFailedTotal.WithLabelValues(channel, severity, errorType).Inc()
}

// RecordAlertSuppressed increments alert suppressed counter
func RecordAlertSuppressed(reason string) {
	alertsSuppressed.WithLabelValues(reason).Inc()
}

// RecordAlertDeliveryDuration records alert delivery duration
func RecordAlertDeliveryDuration(channel string, durationSeconds float64) {
	alertDuration.WithLabelValues(channel).Observe(durationSeconds)
}
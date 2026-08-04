package telemetry

import (
	"context"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registry     *prometheus.Registry
	registryOnce sync.Once

	// Request metrics
	requestsTotal     *prometheus.CounterVec
	requestDuration   *prometheus.HistogramVec
	requestsInFlight  *prometheus.GaugeVec

	// Token metrics
	tokensTotal       *prometheus.CounterVec
	tokensDuration    *prometheus.HistogramVec

	// Security metrics
	piiDetectionsTotal       *prometheus.CounterVec
	contentSafetyDetectionsTotal *prometheus.CounterVec
	alertsSentTotal          *prometheus.CounterVec

	// Error metrics
	errorsTotal       *prometheus.CounterVec

	// System metrics
	uptimeGauge       prometheus.Gauge
	startTimeGauge    prometheus.Gauge
)

// InitMetrics initializes Prometheus metrics
func InitMetrics() *prometheus.Registry {
	registryOnce.Do(func() {
		registry = prometheus.NewRegistry()

		// Request metrics
		requestsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_gateway_requests_total",
				Help: "Total number of requests processed",
			},
			[]string{"provider", "model", "method", "status"},
		)

		requestDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "ai_gateway_request_duration_seconds",
				Help: "Request duration in seconds",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
			},
			[]string{"provider", "model", "method"},
		)

		requestsInFlight = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ai_gateway_requests_in_flight",
				Help: "Number of requests currently being processed",
			},
			[]string{"provider", "model"},
		)

		// Token metrics
		tokensTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_gateway_tokens_total",
				Help: "Total number of tokens processed",
			},
			[]string{"provider", "model", "type"},
		)

		tokensDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "ai_gateway_token_processing_duration_seconds",
				Help: "Time spent processing tokens in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
			},
			[]string{"provider", "model"},
		)

		// Security metrics
		piiDetectionsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_gateway_pii_detections_total",
				Help: "Total number of PII detections",
			},
			[]string{"provider", "model", "pii_type", "action"},
		)

		contentSafetyDetectionsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_gateway_content_safety_detections_total",
				Help: "Total number of content safety detections",
			},
			[]string{"provider", "model", "category", "action"},
		)

		alertsSentTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_gateway_alerts_sent_total",
				Help: "Total number of alerts sent",
			},
			[]string{"channel", "severity", "status"},
		)

		// Error metrics
		errorsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_gateway_errors_total",
				Help: "Total number of errors",
			},
			[]string{"provider", "model", "error_type"},
		)

		// System metrics
		uptimeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ai_gateway_uptime_seconds",
			Help: "Time since the gateway started in seconds",
		})

		startTimeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ai_gateway_start_time_seconds",
			Help: "Unix timestamp when the gateway started",
		})

		// Register all metrics
		registry.MustRegister(
			requestsTotal,
			requestDuration,
			requestsInFlight,
			tokensTotal,
			tokensDuration,
			piiDetectionsTotal,
			contentSafetyDetectionsTotal,
			alertsSentTotal,
			errorsTotal,
			uptimeGauge,
			startTimeGauge,
		)
	})

	return registry
}

// RecordRequest increments request counter
func RecordRequest(provider, model, method, status string) {
	requestsTotal.WithLabelValues(provider, model, method, status).Inc()
}

// RecordRequestDuration records request duration
func RecordRequestDuration(provider, model, method string, duration float64) {
	requestDuration.WithLabelValues(provider, model, method).Observe(duration)
}

// IncRequestsInFlight increments in-flight requests gauge
func IncRequestsInFlight(provider, model string) {
	requestsInFlight.WithLabelValues(provider, model).Inc()
}

// DecRequestsInFlight decrements in-flight requests gauge
func DecRequestsInFlight(provider, model string) {
	requestsInFlight.WithLabelValues(provider, model).Dec()
}

// RecordTokens increments token counter
func RecordTokens(provider, model, tokenType string, count int) {
	tokensTotal.WithLabelValues(provider, model, tokenType).Add(float64(count))
}

// RecordTokenDuration records token processing duration
func RecordTokenDuration(provider, model string, duration float64) {
	tokensDuration.WithLabelValues(provider, model).Observe(duration)
}

// RecordPIIDetection increments PII detection counter
func RecordPIIDetection(provider, model, piiType, action string) {
	piiDetectionsTotal.WithLabelValues(provider, model, piiType, action).Inc()
}

// RecordContentSafetyDetection increments content safety detection counter
func RecordContentSafetyDetection(provider, model, category, action string) {
	contentSafetyDetectionsTotal.WithLabelValues(provider, model, category, action).Inc()
}

// RecordAlertSent increments alert sent counter
func RecordAlertSent(channel, severity, status string) {
	alertsSentTotal.WithLabelValues(channel, severity, status).Inc()
}

// RecordError increments error counter
func RecordError(provider, model, errorType string) {
	errorsTotal.WithLabelValues(provider, model, errorType).Inc()
}

// SetUptime updates uptime gauge
func SetUptime(uptime float64) {
	uptimeGauge.Set(uptime)
}

// SetStartTime sets the start time gauge
func SetStartTime(startTime float64) {
	startTimeGauge.Set(startTime)
}

// MetricsHandler returns HTTP handler for /metrics endpoint
func MetricsHandler() http.Handler {
	return promhttp.HandlerFor(
		GetRegistry(),
		promhttp.HandlerOpts{},
	)
}

// GetRegistry returns the Prometheus registry
func GetRegistry() *prometheus.Registry {
	if registry == nil {
		InitMetrics()
	}
	return registry
}

// MetricsMiddleware returns middleware that records metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract provider and model from context or request
		provider := "unknown"
		model := "unknown"
		method := r.Method

		// Track in-flight requests
		IncRequestsInFlight(provider, model)
		defer DecRequestsInFlight(provider, model)

		// Record start time
		start := time.Now()

		// Call next handler
		next.ServeHTTP(w, r)

		// Record duration
		duration := time.Since(start).Seconds()
		RecordRequestDuration(provider, model, method, duration)

		// Record request
		status := "200" // Default, should be extracted from response
		RecordRequest(provider, model, method, status)
	})
}
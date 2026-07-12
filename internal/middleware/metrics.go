package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"open-ai-gateway/internal/responsestore"
	"open-ai-gateway/internal/routes"
)

type Metrics struct {
	mu                  sync.Mutex
	requests            map[metricKey]requestMetric
	tokens              map[tokenMetricKey]int64
	costs               map[costMetricKey]float64
	rateLimitRejections map[rateLimitRejectionMetricKey]int64
	circuitOpen         map[circuitOpenMetricKey]int64
	fallbacks           map[fallbackMetricKey]int64
	health              map[string]bool
	responseStore       *responsestore.Store
}

func (m *Metrics) SetResponseStore(store *responsestore.Store) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.responseStore = store
	m.mu.Unlock()
}

type metricsFieldsKey struct{}

type MetricsFields struct {
	Client string
}

type metricKey struct {
	Method string
	Path   string
	Status int
	Client string
}

type requestMetric struct {
	Count           int64
	DurationSeconds float64
}

type tokenMetricKey struct {
	Path     string
	Model    string
	Provider string
	Type     string
	Client   string
}

type costMetricKey struct {
	Path     string
	Model    string
	Provider string
	Type     string
	Client   string
}

type rateLimitRejectionMetricKey struct {
	Path   string
	Client string
}

type circuitOpenMetricKey struct {
	Path     string
	Model    string
	Provider string
	Client   string
}

type fallbackMetricKey struct {
	Path         string
	Model        string
	FromProvider string
	ToProvider   string
	Client       string
}

func NewMetrics() *Metrics {
	return &Metrics{
		requests:            make(map[metricKey]requestMetric),
		tokens:              make(map[tokenMetricKey]int64),
		costs:               make(map[costMetricKey]float64),
		rateLimitRejections: make(map[rateLimitRejectionMetricKey]int64),
		circuitOpen:         make(map[circuitOpenMetricKey]int64),
		fallbacks:           make(map[fallbackMetricKey]int64),
		health:              make(map[string]bool),
	}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		fields := &MetricsFields{}
		r = r.WithContext(context.WithValue(r.Context(), metricsFieldsKey{}, fields))
		rec := &metricsRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		client := fields.Client
		if client == "" {
			if routes.IsPublicPath(r.URL.Path) {
				client = "public"
			} else {
				client = "unauthenticated"
			}
		}
		m.Observe(r.Method, routes.NormalizePath(r.URL.Path), rec.status, client, time.Since(started))
	})
}

func SetMetricsClient(ctx context.Context, client string) {
	fields, _ := ctx.Value(metricsFieldsKey{}).(*MetricsFields)
	if fields == nil {
		return
	}
	fields.Client = client
}

func (m *Metrics) Observe(method, path string, status int, client string, duration time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	key := metricKey{Method: method, Path: path, Status: status, Client: client}
	current := m.requests[key]
	current.Count++
	current.DurationSeconds += duration.Seconds()
	m.requests[key] = current
}

func (m *Metrics) ObserveTokens(path, model, providerName, tokenType, client string, tokens int) {
	if m == nil || tokens <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tokenMetricKey{
		Path:     routes.NormalizePath(path),
		Model:    model,
		Provider: providerName,
		Type:     tokenType,
		Client:   client,
	}
	m.tokens[key] += int64(tokens)
}

func (m *Metrics) ObserveTokenCostUSD(path, model, providerName, tokenType, client string, cost float64) {
	if m == nil || cost <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	key := costMetricKey{
		Path:     routes.NormalizePath(path),
		Model:    model,
		Provider: providerName,
		Type:     tokenType,
		Client:   client,
	}
	m.costs[key] += cost
}

func (m *Metrics) ObserveRateLimitRejection(path, client string) {
	if m == nil {
		return
	}
	if client == "" {
		client = "unconfigured"
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	key := rateLimitRejectionMetricKey{
		Path:   routes.NormalizePath(path),
		Client: client,
	}
	m.rateLimitRejections[key]++
}

func (m *Metrics) ObserveProviderCircuitOpen(path, model, providerName, client string) {
	if m == nil || model == "" || providerName == "" {
		return
	}
	if client == "" {
		client = "unconfigured"
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	key := circuitOpenMetricKey{
		Path:     routes.NormalizePath(path),
		Model:    model,
		Provider: providerName,
		Client:   client,
	}
	m.circuitOpen[key]++
}

func (m *Metrics) ObserveProviderFallback(path, model, fromProvider, toProvider, client string) {
	if m == nil || model == "" || fromProvider == "" || toProvider == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fallbackMetricKey{
		Path:         routes.NormalizePath(path),
		Model:        model,
		FromProvider: fromProvider,
		ToProvider:   toProvider,
		Client:       client,
	}
	m.fallbacks[key]++
}

func (m *Metrics) ObserveProviderHealth(providerName string, healthy bool) {
	if m == nil || providerName == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.health[providerName] = healthy
}

func (m *Metrics) WritePrometheus(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if m == nil {
		return
	}

	snapshot := m.snapshot()
	fmt.Fprintln(w, "# HELP open_ai_gateway_http_requests_total Total HTTP requests.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_http_requests_total counter")
	for _, item := range snapshot {
		fmt.Fprintf(w,
			"open_ai_gateway_http_requests_total{method=%q,path=%q,status=%q,client=%q} %d\n",
			item.Key.Method,
			item.Key.Path,
			fmt.Sprint(item.Key.Status),
			item.Key.Client,
			item.Value.Count,
		)
	}

	fmt.Fprintln(w, "# HELP open_ai_gateway_http_request_duration_seconds_total Total HTTP request duration in seconds.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_http_request_duration_seconds_total counter")
	for _, item := range snapshot {
		fmt.Fprintf(w,
			"open_ai_gateway_http_request_duration_seconds_total{method=%q,path=%q,status=%q,client=%q} %.9f\n",
			item.Key.Method,
			item.Key.Path,
			fmt.Sprint(item.Key.Status),
			item.Key.Client,
			item.Value.DurationSeconds,
		)
	}

	tokenSnapshot := m.tokenSnapshot()
	fmt.Fprintln(w, "# HELP open_ai_gateway_tokens_total Total provider-reported tokens.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_tokens_total counter")
	for _, item := range tokenSnapshot {
		fmt.Fprintf(w,
			"open_ai_gateway_tokens_total{path=%q,model=%q,provider=%q,type=%q,client=%q} %d\n",
			item.Key.Path,
			item.Key.Model,
			item.Key.Provider,
			item.Key.Type,
			item.Key.Client,
			item.Value,
		)
	}

	costSnapshot := m.costSnapshot()
	fmt.Fprintln(w, "# HELP open_ai_gateway_token_cost_usd_total Total provider-reported token cost in USD.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_token_cost_usd_total counter")
	for _, item := range costSnapshot {
		fmt.Fprintf(w,
			"open_ai_gateway_token_cost_usd_total{path=%q,model=%q,provider=%q,type=%q,client=%q} %.9f\n",
			item.Key.Path,
			item.Key.Model,
			item.Key.Provider,
			item.Key.Type,
			item.Key.Client,
			item.Value,
		)
	}

	rateLimitRejectionSnapshot := m.rateLimitRejectionSnapshot()
	fmt.Fprintln(w, "# HELP open_ai_gateway_rate_limit_rejections_total Total gateway rate limit rejections.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_rate_limit_rejections_total counter")
	for _, item := range rateLimitRejectionSnapshot {
		fmt.Fprintf(w,
			"open_ai_gateway_rate_limit_rejections_total{path=%q,client=%q} %d\n",
			item.Key.Path,
			item.Key.Client,
			item.Value,
		)
	}

	circuitOpenSnapshot := m.circuitOpenSnapshot()
	fmt.Fprintln(w, "# HELP open_ai_gateway_provider_circuit_open_total Total provider attempts skipped because the circuit was open.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_provider_circuit_open_total counter")
	for _, item := range circuitOpenSnapshot {
		fmt.Fprintf(w,
			"open_ai_gateway_provider_circuit_open_total{path=%q,model=%q,provider=%q,client=%q} %d\n",
			item.Key.Path,
			item.Key.Model,
			item.Key.Provider,
			item.Key.Client,
			item.Value,
		)
	}

	fallbackSnapshot := m.fallbackSnapshot()
	fmt.Fprintln(w, "# HELP open_ai_gateway_provider_fallbacks_total Total provider fallback attempts.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_provider_fallbacks_total counter")
	for _, item := range fallbackSnapshot {
		fmt.Fprintf(w,
			"open_ai_gateway_provider_fallbacks_total{path=%q,model=%q,from_provider=%q,to_provider=%q,client=%q} %d\n",
			item.Key.Path,
			item.Key.Model,
			item.Key.FromProvider,
			item.Key.ToProvider,
			item.Key.Client,
			item.Value,
		)
	}

	healthSnapshot := m.healthSnapshot()
	fmt.Fprintln(w, "# HELP open_ai_gateway_provider_health_status Current provider health status.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_provider_health_status gauge")
	for _, item := range healthSnapshot {
		healthyValue := 0
		unhealthyValue := 1
		if item.Healthy {
			healthyValue = 1
			unhealthyValue = 0
		}
		fmt.Fprintf(w, "open_ai_gateway_provider_health_status{provider=%q,state=%q} %d\n", item.Provider, "healthy", healthyValue)
		fmt.Fprintf(w, "open_ai_gateway_provider_health_status{provider=%q,state=%q} %d\n", item.Provider, "unhealthy", unhealthyValue)
	}

	storeSnapshot := m.responseStoreSnapshot()
	fmt.Fprintln(w, "# HELP open_ai_gateway_response_store_entries Current stored response entries.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_response_store_entries gauge")
	fmt.Fprintf(w, "open_ai_gateway_response_store_entries %d\n", storeSnapshot.Entries)
	fmt.Fprintln(w, "# HELP open_ai_gateway_response_store_bytes Current stored response transcript bytes.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_response_store_bytes gauge")
	fmt.Fprintf(w, "open_ai_gateway_response_store_bytes %d\n", storeSnapshot.Bytes)
	fmt.Fprintln(w, "# HELP open_ai_gateway_response_store_evictions_total Total response store evictions.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_response_store_evictions_total counter")
	for _, reason := range []responsestore.EvictionReason{responsestore.EvictionExpired, responsestore.EvictionCapacity} {
		fmt.Fprintf(w, "open_ai_gateway_response_store_evictions_total{reason=%q} %d\n", reason, storeSnapshot.Evictions[reason])
	}
	fmt.Fprintln(w, "# HELP open_ai_gateway_response_store_misses_total Total response store lookup misses.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_response_store_misses_total counter")
	for _, reason := range []responsestore.MissReason{responsestore.MissNotFound, responsestore.MissExpired, responsestore.MissClient, responsestore.MissModel} {
		fmt.Fprintf(w, "open_ai_gateway_response_store_misses_total{reason=%q} %d\n", reason, storeSnapshot.Misses[reason])
	}
}

func (m *Metrics) responseStoreSnapshot() responsestore.Stats {
	m.mu.Lock()
	store := m.responseStore
	m.mu.Unlock()
	if store == nil {
		return responsestore.Stats{Evictions: map[responsestore.EvictionReason]uint64{}, Misses: map[responsestore.MissReason]uint64{}}
	}
	return store.Snapshot()
}

type metricSnapshotItem struct {
	Key   metricKey
	Value requestMetric
}

func (m *Metrics) snapshot() []metricSnapshotItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]metricSnapshotItem, 0, len(m.requests))
	for key, value := range m.requests {
		items = append(items, metricSnapshotItem{Key: key, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		left := items[i].Key
		right := items[j].Key
		if cmp := strings.Compare(left.Path, right.Path); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.Method, right.Method); cmp != 0 {
			return cmp < 0
		}
		if left.Status != right.Status {
			return left.Status < right.Status
		}
		return left.Client < right.Client
	})
	return items
}

type tokenMetricSnapshotItem struct {
	Key   tokenMetricKey
	Value int64
}

func (m *Metrics) tokenSnapshot() []tokenMetricSnapshotItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]tokenMetricSnapshotItem, 0, len(m.tokens))
	for key, value := range m.tokens {
		items = append(items, tokenMetricSnapshotItem{Key: key, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		left := items[i].Key
		right := items[j].Key
		if cmp := strings.Compare(left.Path, right.Path); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.Model, right.Model); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.Provider, right.Provider); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.Type, right.Type); cmp != 0 {
			return cmp < 0
		}
		return left.Client < right.Client
	})
	return items
}

type costMetricSnapshotItem struct {
	Key   costMetricKey
	Value float64
}

func (m *Metrics) costSnapshot() []costMetricSnapshotItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]costMetricSnapshotItem, 0, len(m.costs))
	for key, value := range m.costs {
		items = append(items, costMetricSnapshotItem{Key: key, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		left := items[i].Key
		right := items[j].Key
		if cmp := strings.Compare(left.Path, right.Path); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.Model, right.Model); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.Provider, right.Provider); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.Type, right.Type); cmp != 0 {
			return cmp < 0
		}
		return left.Client < right.Client
	})
	return items
}

type fallbackMetricSnapshotItem struct {
	Key   fallbackMetricKey
	Value int64
}

type rateLimitRejectionMetricSnapshotItem struct {
	Key   rateLimitRejectionMetricKey
	Value int64
}

type circuitOpenMetricSnapshotItem struct {
	Key   circuitOpenMetricKey
	Value int64
}

func (m *Metrics) rateLimitRejectionSnapshot() []rateLimitRejectionMetricSnapshotItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]rateLimitRejectionMetricSnapshotItem, 0, len(m.rateLimitRejections))
	for key, value := range m.rateLimitRejections {
		items = append(items, rateLimitRejectionMetricSnapshotItem{Key: key, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		left := items[i].Key
		right := items[j].Key
		if cmp := strings.Compare(left.Path, right.Path); cmp != 0 {
			return cmp < 0
		}
		return left.Client < right.Client
	})
	return items
}

func (m *Metrics) circuitOpenSnapshot() []circuitOpenMetricSnapshotItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]circuitOpenMetricSnapshotItem, 0, len(m.circuitOpen))
	for key, value := range m.circuitOpen {
		items = append(items, circuitOpenMetricSnapshotItem{Key: key, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		left := items[i].Key
		right := items[j].Key
		if cmp := strings.Compare(left.Path, right.Path); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.Model, right.Model); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.Provider, right.Provider); cmp != 0 {
			return cmp < 0
		}
		return left.Client < right.Client
	})
	return items
}

func (m *Metrics) fallbackSnapshot() []fallbackMetricSnapshotItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]fallbackMetricSnapshotItem, 0, len(m.fallbacks))
	for key, value := range m.fallbacks {
		items = append(items, fallbackMetricSnapshotItem{Key: key, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		left := items[i].Key
		right := items[j].Key
		if cmp := strings.Compare(left.Path, right.Path); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.Model, right.Model); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.FromProvider, right.FromProvider); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(left.ToProvider, right.ToProvider); cmp != 0 {
			return cmp < 0
		}
		return left.Client < right.Client
	})
	return items
}

type healthMetricSnapshotItem struct {
	Provider string
	Healthy  bool
}

func (m *Metrics) healthSnapshot() []healthMetricSnapshotItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]healthMetricSnapshotItem, 0, len(m.health))
	for providerName, healthy := range m.health {
		items = append(items, healthMetricSnapshotItem{Provider: providerName, Healthy: healthy})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Provider < items[j].Provider
	})
	return items
}

type metricsRecorder struct {
	http.ResponseWriter
	status int
}

func (r *metricsRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *metricsRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

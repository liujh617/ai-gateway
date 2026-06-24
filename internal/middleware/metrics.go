package middleware

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu       sync.Mutex
	requests map[metricKey]requestMetric
}

type metricKey struct {
	Method string
	Path   string
	Status int
}

type requestMetric struct {
	Count           int64
	DurationSeconds float64
}

func NewMetrics() *Metrics {
	return &Metrics{
		requests: make(map[metricKey]requestMetric),
	}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rec := &metricsRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		m.Observe(r.Method, r.URL.Path, rec.status, time.Since(started))
	})
}

func (m *Metrics) Observe(method, path string, status int, duration time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	key := metricKey{Method: method, Path: path, Status: status}
	current := m.requests[key]
	current.Count++
	current.DurationSeconds += duration.Seconds()
	m.requests[key] = current
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
			"open_ai_gateway_http_requests_total{method=%q,path=%q,status=%q} %d\n",
			item.Key.Method,
			item.Key.Path,
			fmt.Sprint(item.Key.Status),
			item.Value.Count,
		)
	}

	fmt.Fprintln(w, "# HELP open_ai_gateway_http_request_duration_seconds_total Total HTTP request duration in seconds.")
	fmt.Fprintln(w, "# TYPE open_ai_gateway_http_request_duration_seconds_total counter")
	for _, item := range snapshot {
		fmt.Fprintf(w,
			"open_ai_gateway_http_request_duration_seconds_total{method=%q,path=%q,status=%q} %.9f\n",
			item.Key.Method,
			item.Key.Path,
			fmt.Sprint(item.Key.Status),
			item.Value.DurationSeconds,
		)
	}
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
		return left.Status < right.Status
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

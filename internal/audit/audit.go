package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"open-ai-gateway/internal/requestctx"
)

const TraceIDHeader = "X-Agent-Trace-Id"

const (
	EventRequest     = "request"
	EventResponse    = "response"
	EventStreamChunk = "stream_chunk"
	EventStreamDone  = "stream_done"
	EventError       = "error"
)

type Event struct {
	Timestamp     time.Time       `json:"timestamp"`
	Event         string          `json:"event"`
	RequestID     string          `json:"request_id,omitempty"`
	TraceID       string          `json:"trace_id,omitempty"`
	Path          string          `json:"path,omitempty"`
	Client        string          `json:"client,omitempty"`
	ExternalModel string          `json:"external_model,omitempty"`
	Provider      string          `json:"provider,omitempty"`
	UpstreamModel string          `json:"upstream_model,omitempty"`
	Status        int             `json:"status,omitempty"`
	DurationMS    int64           `json:"duration_ms,omitempty"`
	Body          json.RawMessage `json:"body,omitempty"`
	Error         string          `json:"error,omitempty"`
}

type Recorder interface {
	Record(ctx context.Context, event Event)
	Close() error
}

type NoopRecorder struct{}

func (NoopRecorder) Record(context.Context, Event) {}

func (NoopRecorder) Close() error {
	return nil
}

type JSONLRecorder struct {
	mu     sync.Mutex
	file   *os.File
	logger *slog.Logger
	now    func() time.Time
}

func NewJSONLRecorder(path string) (*JSONLRecorder, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &JSONLRecorder{
		file:   file,
		logger: slog.Default(),
		now:    time.Now,
	}, nil
}

func (r *JSONLRecorder) Record(ctx context.Context, event Event) {
	if r == nil || r.file == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = r.now().UTC()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		r.logger.Debug("failed to marshal audit event", "error", err)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.file.Write(append(payload, '\n')); err != nil {
		r.logger.Debug("failed to write audit event", "error", err)
	}
}

func (r *JSONLRecorder) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	err := r.file.Close()
	r.file = nil
	return err
}

func TraceIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if traceID := strings.TrimSpace(r.Header.Get(TraceIDHeader)); traceID != "" {
		return traceID
	}
	return requestctx.RequestID(r.Context())
}

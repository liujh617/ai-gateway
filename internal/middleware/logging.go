package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"open-ai-gateway/internal/routes"
)

type logFieldsKey struct{}

type LogFields struct {
	Client           string
	ExternalModel    string
	Provider         string
	UpstreamModel    string
	Stream           *bool
	ErrorType        string
	ErrorCode        string
	PreviousResponse *bool
}

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			fields := &LogFields{}
			r = r.WithContext(context.WithValue(r.Context(), logFieldsKey{}, fields))
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			attrs := []any{
				"request_id", RequestIDFromContext(r.Context()),
				"method", r.Method,
				"path", routes.NormalizePath(r.URL.Path),
				"status", rec.status,
				"latency_ms", time.Since(started).Milliseconds(),
			}
			if fields.ExternalModel != "" {
				attrs = append(attrs, "external_model", fields.ExternalModel)
			}
			if fields.Client != "" {
				attrs = append(attrs, "client", fields.Client)
			}
			if fields.Provider != "" {
				attrs = append(attrs, "provider", fields.Provider)
			}
			if fields.UpstreamModel != "" {
				attrs = append(attrs, "upstream_model", fields.UpstreamModel)
			}
			if fields.Stream != nil {
				attrs = append(attrs, "stream", *fields.Stream)
			}
			if fields.PreviousResponse != nil {
				attrs = append(attrs, "previous_response", *fields.PreviousResponse)
			}
			if fields.ErrorType != "" {
				attrs = append(attrs, "error_type", fields.ErrorType)
			}
			if fields.ErrorCode != "" {
				attrs = append(attrs, "error_code", fields.ErrorCode)
			}
			logger.Info("request completed", attrs...)
		})
	}
}

func SetLogClient(ctx context.Context, client string) {
	fields := logFieldsFromContext(ctx)
	if fields == nil {
		return
	}
	fields.Client = client
}

func SetLogRoute(ctx context.Context, externalModel, providerName, upstreamModel string) {
	fields := logFieldsFromContext(ctx)
	if fields == nil {
		return
	}
	fields.ExternalModel = externalModel
	fields.Provider = providerName
	fields.UpstreamModel = upstreamModel
}

func SetLogStream(ctx context.Context, stream bool) {
	fields := logFieldsFromContext(ctx)
	if fields == nil {
		return
	}
	fields.Stream = &stream
}

func SetLogPreviousResponse(ctx context.Context, previous bool) {
	fields := logFieldsFromContext(ctx)
	if fields == nil {
		return
	}
	fields.PreviousResponse = &previous
}

func SetLogError(ctx context.Context, typ string, code *string) {
	fields := logFieldsFromContext(ctx)
	if fields == nil {
		return
	}
	fields.ErrorType = typ
	if code != nil {
		fields.ErrorCode = *code
	}
}

func logFieldsFromContext(ctx context.Context) *LogFields {
	fields, _ := ctx.Value(logFieldsKey{}).(*LogFields)
	return fields
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

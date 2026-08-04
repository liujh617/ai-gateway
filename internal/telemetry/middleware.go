package telemetry

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware returns middleware that creates spans for each request
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Start span
		ctx, span := StartHTTPSpan(ctx, r.Method, r.URL.Path)
		defer span.End()

		// Extract trace context from headers
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

		// Add standard HTTP attributes
		SetSpanAttributes(ctx,
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.host", r.Host),
			attribute.String("http.scheme", r.URL.Scheme),
			attribute.String("http.user_agent", r.UserAgent()),
			attribute.Int("http.content_length", int(r.ContentLength)),
		)

		// Create response wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call next handler with updated context
		next.ServeHTTP(rw, r.WithContext(ctx))

		// Set span status
		if rw.statusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(rw.statusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}

		// Add response attributes
		span.SetAttributes(
			attribute.Int("http.status_code", rw.statusCode),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// TraceUpstreamCall traces an upstream API call
func TraceUpstreamCall(ctx context.Context, provider, model string, fn func(context.Context) error) error {
	ctx, span := StartUpstreamSpan(ctx, provider, model)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start).Seconds()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		RecordError(provider, model, "upstream_error")
	} else {
		span.SetStatus(codes.Ok, "")
	}

	// Record duration
	RecordRequestDuration(provider, model, "upstream", duration)

	return err
}

// TracePIIDetection traces PII detection
func TracePIIDetection(ctx context.Context, provider, model, piiType, action string, fn func() error) error {
	ctx, span := StartPIISpan(ctx, piiType)
	defer span.End()

	err := fn()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
		RecordPIIDetection(provider, model, piiType, action)
	}

	span.SetAttributes(
		attribute.String("pii.action", action),
	)

	return err
}

// TraceContentSafetyDetection traces content safety detection
func TraceContentSafetyDetection(ctx context.Context, provider, model, category, action string, fn func() error) error {
	ctx, span := StartContentSafetySpan(ctx, category)
	defer span.End()

	err := fn()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
		RecordContentSafetyDetection(provider, model, category, action)
	}

	span.SetAttributes(
		attribute.String("content_safety.action", action),
	)

	return err
}

// TraceAuditLog traces audit log write
func TraceAuditLog(ctx context.Context, hasPII, hasContentSafety bool, fn func() error) error {
	ctx, span := StartAuditSpan(ctx)
	defer span.End()

	err := fn()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.Bool("audit.has_pii", hasPII),
		attribute.Bool("audit.has_content_safety", hasContentSafety),
	)

	return err
}
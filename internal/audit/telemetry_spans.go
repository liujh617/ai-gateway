package audit

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// SpanContext provides telemetry span context for audit operations
type SpanContext struct {
	ctx     context.Context
	span    trace.Span
	enabled bool
}

// StartAuditSpan starts a new audit span for tracing
func StartAuditSpan(ctx context.Context, event *Event) (context.Context, trace.Span) {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer("ai-gateway/audit")

	spanName := "audit.record"
	if event.Event != "" {
		spanName = "audit." + event.Event
	}

	attrs := []attribute.KeyValue{
		attribute.String("audit.event_type", event.Event),
		attribute.String("audit.request_id", event.RequestID),
		attribute.String("audit.client", event.Client),
		attribute.String("audit.provider", event.Provider),
		attribute.String("audit.model", event.Model),
	}

	if event.StatusCode > 0 {
		attrs = append(attrs, attribute.Int("audit.status_code", event.StatusCode))
	}

	return tracer.Start(ctx, spanName, trace.WithAttributes(attrs...))
}

// StartPIIDetectionSpan starts a new span for PII detection
func StartPIIDetectionSpan(ctx context.Context, requestID string) (context.Context, trace.Span) {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer("ai-gateway/audit")

	return tracer.Start(ctx, "pii.detect",
		trace.WithAttributes(
			attribute.String("audit.request_id", requestID),
			attribute.String("component", "pii_detector"),
		),
	)
}

// SetPIISpanAttributes sets PII detection results on span
func SetPIISpanAttributes(span trace.Span, result *PIIDetectionResult, action PIIDetectionAction) {
	if !span.IsRecording() {
		return
	}

	span.SetAttributes(
		attribute.Bool("pii.has_pii", result.HasPII),
		attribute.Int("pii.total_count", result.TotalCount),
		attribute.String("pii.action", string(action)),
	)

	for piiType, count := range result.TypeCounts {
		span.SetAttributes(attribute.Int("pii.count."+string(piiType), count))
	}
}

// StartContentSafetyDetectionSpan starts a new span for content safety detection
func StartContentSafetyDetectionSpan(ctx context.Context, requestID string) (context.Context, trace.Span) {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer("ai-gateway/audit")

	return tracer.Start(ctx, "content_safety.detect",
		trace.WithAttributes(
			attribute.String("audit.request_id", requestID),
			attribute.String("component", "content_safety_detector"),
		),
	)
}

// SetContentSafetySpanAttributes sets content safety detection results on span
func SetContentSafetySpanAttributes(span trace.Span, result *ContentSafetyResult, action ContentSafetyAction) {
	if !span.IsRecording() {
		return
	}

	span.SetAttributes(
		attribute.Bool("content_safety.has_violation", result.HasViolation),
		attribute.Int("content_safety.total_count", result.TotalCount),
		attribute.String("content_safety.action", string(action)),
	)

	for category, violations := range result.CategoryViolations {
		span.SetAttributes(attribute.Int("content_safety.count."+string(category), len(violations)))
	}
}

// StartEncryptionSpan starts a new span for encryption operations
func StartEncryptionSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer("ai-gateway/audit")

	return tracer.Start(ctx, "audit.encrypt",
		trace.WithAttributes(
			attribute.String("encryption.operation", operation),
			attribute.String("component", "encryption"),
		),
	)
}

// SetEncryptionSpanAttributes sets encryption operation results on span
func SetEncryptionSpanAttributes(span trace.Span, success bool, durationMs float64) {
	if !span.IsRecording() {
		return
	}

	span.SetAttributes(
		attribute.Bool("encryption.success", success),
		attribute.Float64("encryption.duration_ms", durationMs),
	)
}
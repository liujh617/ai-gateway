package telemetry

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	globalTracer     trace.Tracer
	globalTracerOnce sync.Once
	globalProvider   *sdktrace.TracerProvider
)

// Config holds telemetry configuration
type Config struct {
	Enabled       bool    `json:"enabled"`
	ServiceName   string  `json:"service_name"`
	ServiceVersion string `json:"service_version"`
	Tracing       TracingConfig `json:"tracing"`
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	Enabled     bool    `json:"enabled"`
	SampleRate  float64 `json:"sample_rate"`
	Exporter    string  `json:"exporter"` // "otlp", "stdout", "noop"
	Endpoint    string  `json:"endpoint"`
	Insecure    bool    `json:"insecure"`
	Headers     map[string]string `json:"headers"`
}

// InitTracer initializes the global tracer provider
func InitTracer(cfg *Config) (func(context.Context) error, error) {
	if !cfg.Enabled || !cfg.Tracing.Enabled {
		// Noop tracer
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		globalTracer = trace.NewNoopTracerProvider().Tracer("")
		return func(ctx context.Context) error { return nil }, nil
	}

	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	// Create exporter
	exporter, err := createExporter(cfg.Tracing)
	if err != nil {
		return nil, fmt.Errorf("create exporter: %w", err)
	}

	// Create sampler
	sampler := createSampler(cfg.Tracing.SampleRate)

	// Create tracer provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)),
	)

	globalProvider = provider
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Set global tracer
	globalTracer = provider.Tracer("ai-gateway")

	// Return shutdown function
	return provider.Shutdown, nil
}

// createExporter creates a trace exporter based on configuration
func createExporter(cfg TracingConfig) (sdktrace.SpanExporter, error) {
	switch cfg.Exporter {
	case "otlp":
		if cfg.Insecure {
			// Use HTTP exporter (insecure)
			client := otlptracehttp.NewClient(
				otlptracehttp.WithEndpoint(cfg.Endpoint),
				otlptracehttp.WithInsecure(),
			)
			return otlptrace.New(context.Background(), client)
		}
		// Use gRPC exporter (default)
		client := otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
			otlptracegrpc.WithHeaders(cfg.Headers),
		)
		return otlptrace.New(context.Background(), client)

	case "stdout":
		// Stdout exporter for debugging
		return sdktrace.NewConsoleExporter(os.Stdout), nil

	case "noop", "":
		// Noop exporter
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown exporter type: %s", cfg.Exporter)
	}
}

// createSampler creates a sampler based on sample rate
func createSampler(rate float64) sdktrace.Sampler {
	if rate <= 0.0 {
		return sdktrace.NeverSample()
	}
	if rate >= 1.0 {
		return sdktrace.AlwaysSample()
	}
	return sdktrace.TraceIDRatioBased(rate)
}

// GetTracer returns the global tracer instance
func GetTracer() trace.Tracer {
	globalTracerOnce.Do(func() {
		if globalTracer == nil {
			// Fallback to noop tracer
			globalTracer = trace.NewNoopTracerProvider().Tracer("")
		}
	})
	return globalTracer
}

// StartSpan starts a new span with the given name and options
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return GetTracer().Start(ctx, name, opts...)
}

// StartHTTPSpan starts a new HTTP span
func StartHTTPSpan(ctx context.Context, method, path string) (context.Context, trace.Span) {
	return StartSpan(ctx, fmt.Sprintf("HTTP %s %s", method, path),
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", path),
		),
	)
}

// StartUpstreamSpan starts a new upstream span
func StartUpstreamSpan(ctx context.Context, provider, model string) (context.Context, trace.Span) {
	return StartSpan(ctx, fmt.Sprintf("upstream.%s", provider),
		trace.WithAttributes(
			attribute.String("provider", provider),
			attribute.String("model", model),
		),
	)
}

// StartAuditSpan starts a new audit span
func StartAuditSpan(ctx context.Context) (context.Context, trace.Span) {
	return StartSpan(ctx, "audit.log")
}

// StartPIISpan starts a new PII detection span
func StartPIISpan(ctx context.Context, piiType string) (context.Context, trace.Span) {
	return StartSpan(ctx, "pii.detect",
		trace.WithAttributes(
			attribute.String("pii.type", piiType),
		),
	)
}

// StartContentSafetySpan starts a new content safety detection span
func StartContentSafetySpan(ctx context.Context, category string) (context.Context, trace.Span) {
	return StartSpan(ctx, "content_safety.detect",
		trace.WithAttributes(
			attribute.String("content_safety.category", category),
		),
	)
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
	}
}

// SetSpanAttributes sets attributes on the current span
func SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// SetSpanStatus sets the status of the current span
func SetSpanStatus(ctx context.Context, code int, message string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		if code >= 400 {
			span.SetStatus(codes.Error, message)
		} else {
			span.SetStatus(codes.Ok, message)
		}
	}
}

// TraceIDFromContext extracts the trace ID from context
func TraceIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanIDFromContext extracts the span ID from context
func SpanIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}